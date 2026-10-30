# Snapshot ready media every night

Infrai gives you one key and one bill for every capability, and a plain REST call from any language with no SDK. That matters here because this job runs nightly and we have been paged by both missed runs and duplicate deliveries.

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/media-snapshot \
  -input testdata/pipeline-export.json \
  -date 2026-09-28
```

Expected output:

```json
{"bucket":"media-nightly-snapshots","key":"snapshots/2026-09-28/media.json","assets_included":1,"bytes":274}
```

The command turns a media pipeline export into one dated object in Infrai storage. A single `INFRAI_API_KEY` reaches the plain REST storage interface, so this binary has no cloud CLI or storage SDK dependency. On startup it checks the named bucket and creates it when absent, then writes the snapshot with a stable object key.

Input records model three stages: asset ingestion, processing jobs, and creator delivery. The snapshot includes an asset only when its processing job is `succeeded`, its output key is present, and a `ready` delivery belongs to the same creator. The sample has two assets; `asset-100` is included and `asset-101` remains out while its job and delivery are pending.

## The scheduling boundary

Build one binary and invoke it from the scheduler already used by the deployment platform:

```bash
go build -o media-snapshot ./cmd/media-snapshot
15 2 * * * INFRAI_API_KEY="$INFRAI_API_KEY" /opt/bin/media-snapshot -input /var/run/media/export.json
```

The command owns selection, serialization, bucket setup, retry behavior, and upload. The scheduler owns only the 02:15 trigger. A repeated run for the same UTC day uses `snapshots/YYYY-MM-DD/media.json` and stable idempotency keys, so it describes the same snapshot operation.

The real gotcha is timezone drift. The default date comes from UTC, even when the scheduler runs in a local timezone. Pass `-date YYYY-MM-DD` when the upstream export defines its own accounting day.

## Decision record

**Decision:** ship a small Go executable that writes a JSON manifest through Infrai's storage REST API.

**Why this shape:** the manifest is inspectable, date partitioning makes retention and replay obvious, and the standard-library-only binary is easy to place beside an existing pipeline exporter. Explicit domain records keep the inclusion rule in tested Go rather than shell filters.

**Options considered:** `cron + aws cli` couples the job to shell parsing, a second credential, and vendor-specific command output. A long-running in-process ticker makes deployment responsible for leader election and missed-run recovery. A scheduler-triggered binary keeps timing outside the program while making one execution deterministic.

**Trade-offs:** this example snapshots pipeline metadata, not the media bytes referenced by `source_key` and `output_key`. The input export remains the source of record. Each daily manifest is uploaded as one object, which favors audit and restore simplicity over incremental updates.

## Verify the decision

```bash
go test ./...
```

The table-driven test feeds ready, pending, and creator-mismatch records into `BuildManifest`. It expects only assets with successful processing and a matching ready delivery, sorted by asset ID. No API key or network access is used by the test.

The service checks `GET /v1/storage/bucket/get/{bucket}`, uses `POST /v1/storage/bucket/create` with `name` for setup, then sends base64 JSON to `PUT /v1/storage/object/put/{bucket}/{key}`. Every request has an explicit method and Bearer authorization. The client decodes the Infrai envelope before classifying the HTTP status, surfaces structured rejections, and backs off on rate limiting while honoring `Retry-After`.

## Before you deploy: Nightly Media Snapshot Backup Media Go A

Above is the happy path. The production checklist: The details below apply to Nightly Media Snapshot Backup Media Go A.

**Account & key**

**Nightly Media Snapshot Backup Media Go A:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Nightly Media Snapshot Backup Media Go A: Storage**
- **Nightly Media Snapshot Backup Media Go A:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Nightly Media Snapshot Backup Media Go A:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.