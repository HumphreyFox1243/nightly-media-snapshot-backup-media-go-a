package snapshot

import (
	"testing"
	"time"
)

func TestBuildManifestSelectsDeliverableProcessedAssets(t *testing.T) {
	tests := []struct {
		name  string
		input PipelineExport
		want  []string
	}{
		{
			name: "includes succeeded job with matching ready delivery",
			input: PipelineExport{
				Assets:     []Asset{{ID: "asset-b", CreatorID: "creator-1"}, {ID: "asset-a", CreatorID: "creator-1"}},
				Jobs:       []ProcessingJob{{AssetID: "asset-b", Status: "succeeded", OutputKey: "ready/b.mp4"}, {AssetID: "asset-a", Status: "succeeded", OutputKey: "ready/a.mp4"}},
				Deliveries: []Delivery{{AssetID: "asset-b", CreatorID: "creator-1", Status: "ready"}, {AssetID: "asset-a", CreatorID: "creator-1", Status: "ready"}},
			},
			want: []string{"asset-a", "asset-b"},
		},
		{
			name: "excludes pending processing and delivery",
			input: PipelineExport{
				Assets:     []Asset{{ID: "processing", CreatorID: "creator-1"}, {ID: "review", CreatorID: "creator-1"}},
				Jobs:       []ProcessingJob{{AssetID: "processing", Status: "running"}, {AssetID: "review", Status: "succeeded", OutputKey: "ready/review.mp4"}},
				Deliveries: []Delivery{{AssetID: "processing", CreatorID: "creator-1", Status: "ready"}, {AssetID: "review", CreatorID: "creator-1", Status: "review"}},
			},
			want: nil,
		},
		{
			name: "excludes creator mismatch",
			input: PipelineExport{
				Assets:     []Asset{{ID: "asset-a", CreatorID: "creator-1"}},
				Jobs:       []ProcessingJob{{AssetID: "asset-a", Status: "succeeded", OutputKey: "ready/a.mp4"}},
				Deliveries: []Delivery{{AssetID: "asset-a", CreatorID: "creator-2", Status: "ready"}},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildManifest(tt.input, time.Date(2026, 9, 28, 23, 30, 0, 0, time.FixedZone("east", 8*60*60)))
			if got.SnapshotDate != "2026-09-28" {
				t.Fatalf("snapshot date = %q", got.SnapshotDate)
			}
			if len(got.Assets) != len(tt.want) {
				t.Fatalf("asset count = %d, want %d", len(got.Assets), len(tt.want))
			}
			for i, id := range tt.want {
				if got.Assets[i].ID != id {
					t.Errorf("asset[%d] = %q, want %q", i, got.Assets[i].ID, id)
				}
			}
		})
	}
}
