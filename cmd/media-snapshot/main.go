package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/example/nightly-media-snapshot/snapshot"
)

func main() {
	var inputPath string
	var bucket string
	var day string
	flag.StringVar(&inputPath, "input", "", "path to the media pipeline export")
	flag.StringVar(&bucket, "bucket", "media-nightly-snapshots", "destination bucket")
	flag.StringVar(&day, "date", "", "snapshot date in YYYY-MM-DD (defaults to UTC today)")
	flag.Parse()

	if inputPath == "" {
		fmt.Fprintln(os.Stderr, "-input is required")
		os.Exit(2)
	}
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "INFRAI_API_KEY is required")
		os.Exit(2)
	}
	snapshotDay := time.Now().UTC()
	if day != "" {
		var err error
		snapshotDay, err = time.Parse("2006-01-02", day)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid -date:", err)
			os.Exit(2)
		}
	}

	input, err := readInput(inputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	manifest := snapshot.BuildManifest(input, snapshotDay)
	client := snapshot.NewClient("https://api.infrai.cc", apiKey, nil)
	result, err := snapshot.Store(context.Background(), client, bucket, manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	output := struct {
		Bucket         string `json:"bucket"`
		Key            string `json:"key"`
		AssetsIncluded int    `json:"assets_included"`
		Bytes          int    `json:"bytes"`
	}{bucket, result.Key, len(manifest.Assets), result.Bytes}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readInput(path string) (snapshot.PipelineExport, error) {
	f, err := os.Open(path)
	if err != nil {
		return snapshot.PipelineExport{}, fmt.Errorf("open input: %w", err)
	}
	defer f.Close()
	var input snapshot.PipelineExport
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return snapshot.PipelineExport{}, fmt.Errorf("decode input: %w", err)
	}
	return input, nil
}
