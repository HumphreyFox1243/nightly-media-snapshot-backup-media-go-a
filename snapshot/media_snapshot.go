package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

type PipelineExport struct {
	Assets     []Asset         `json:"assets"`
	Jobs       []ProcessingJob `json:"jobs"`
	Deliveries []Delivery      `json:"deliveries"`
}

type Asset struct {
	ID        string `json:"id"`
	CreatorID string `json:"creator_id"`
	MediaType string `json:"media_type"`
	SourceKey string `json:"source_key"`
}

type ProcessingJob struct {
	AssetID   string `json:"asset_id"`
	Status    string `json:"status"`
	OutputKey string `json:"output_key"`
}

type Delivery struct {
	AssetID   string `json:"asset_id"`
	CreatorID string `json:"creator_id"`
	Status    string `json:"status"`
}

type SnapshotAsset struct {
	ID        string `json:"id"`
	CreatorID string `json:"creator_id"`
	MediaType string `json:"media_type"`
	SourceKey string `json:"source_key"`
	OutputKey string `json:"output_key"`
	Delivery  string `json:"delivery"`
}

type Manifest struct {
	SnapshotDate string          `json:"snapshot_date"`
	Assets       []SnapshotAsset `json:"assets"`
}

type StoreResult struct {
	Key   string
	Bytes int
}

type objectPutRequest struct {
	DataBase64     string `json:"data_base64"`
	ContentType    string `json:"content_type"`
	IdempotencyKey string `json:"idempotency_key"`
}

type Storage interface {
	StorageBucketGet(context.Context, string) error
	StorageBucketCreate(context.Context, string, string) error
	StorageObjectPut(context.Context, string, string, objectPutRequest) error
}

func BuildManifest(input PipelineExport, at time.Time) Manifest {
	succeeded := make(map[string]string, len(input.Jobs))
	for _, job := range input.Jobs {
		if job.Status == "succeeded" && job.OutputKey != "" {
			succeeded[job.AssetID] = job.OutputKey
		}
	}
	ready := make(map[string]Delivery, len(input.Deliveries))
	for _, delivery := range input.Deliveries {
		if delivery.Status == "ready" {
			ready[delivery.AssetID] = delivery
		}
	}

	manifest := Manifest{SnapshotDate: at.UTC().Format("2006-01-02")}
	for _, asset := range input.Assets {
		outputKey, processed := succeeded[asset.ID]
		delivery, deliverable := ready[asset.ID]
		if !processed || !deliverable || delivery.CreatorID != asset.CreatorID {
			continue
		}
		manifest.Assets = append(manifest.Assets, SnapshotAsset{
			ID: asset.ID, CreatorID: asset.CreatorID, MediaType: asset.MediaType,
			SourceKey: asset.SourceKey, OutputKey: outputKey, Delivery: delivery.Status,
		})
	}
	sort.Slice(manifest.Assets, func(i, j int) bool { return manifest.Assets[i].ID < manifest.Assets[j].ID })
	return manifest
}

func Store(ctx context.Context, storage Storage, bucket string, manifest Manifest) (StoreResult, error) {
	key := fmt.Sprintf("snapshots/%s/media.json", manifest.SnapshotDate)
	if err := storage.StorageBucketGet(ctx, bucket); err != nil {
		var rejection *InfraiError
		if !errors.As(err, &rejection) || rejection.HTTPStatus != 404 {
			return StoreResult{}, fmt.Errorf("inspect bucket: %w", err)
		}
		bucketToken := stableToken("bucket", bucket)
		if err := storage.StorageBucketCreate(ctx, bucket, bucketToken); err != nil {
			return StoreResult{}, fmt.Errorf("prepare bucket: %w", err)
		}
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return StoreResult{}, fmt.Errorf("encode manifest: %w", err)
	}
	request := objectPutRequest{
		DataBase64: base64.StdEncoding.EncodeToString(payload), ContentType: "application/json",
		IdempotencyKey: stableToken(bucket, key),
	}
	if err := storage.StorageObjectPut(ctx, bucket, key, request); err != nil {
		return StoreResult{}, fmt.Errorf("store manifest: %w", err)
	}
	return StoreResult{Key: key, Bytes: len(payload)}, nil
}

func stableToken(parts ...string) string {
	sum := sha256.Sum256([]byte(fmt.Sprint(parts)))
	return "media-snapshot-" + hex.EncodeToString(sum[:12])
}
