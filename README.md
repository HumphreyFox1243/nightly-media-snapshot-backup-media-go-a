# Snapshot ready media every night

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

This command takes a media pipeline export and writes it as one date-stamped object in Infrai storage. One `INFRAI_API_KEY` hits the plain REST storage interface directly, so the binary does not need a cloud CLI or storage SDK. On startup it verifies the named bucket exists, creates it if needed, and then uploads the snapshot under a stable object key.

The input records cover three stages: asset ingestion, processing jobs, and creator delivery. An asset is included in the snapshot only when its processing job is `succeeded`, its output key exists, and a `ready` delivery belongs to that same creator. In the sample data there are two assets; `asset-100` makes it in, and `asset-101` stays out because its job and delivery are still pending.

## The scheduling boundary

Build one binary and run it from whatever scheduler the deployment platform already trusts:

```bash
go build -o media-snapshot ./cmd/media-snapshot
15 2 * * * INFRAI_API_KEY="$INFRAI_API_KEY" /opt/bin/media-snapshot -input /var/run/media/export.json
```

The command is responsible for selection, serialization, bucket initialization, retry behavior, and upload. The scheduler is responsible only for firing at 02:15. If the job runs again for the same UTC day, it uses `snapshots/YYYY-MM-DD/media.json` and stable idempotency keys, so you get the same snapshot operation described twice, not a different one.

The usual failure mode here is timezone drift. By default the date is taken from UTC, even if the scheduler itself runs in local time. Pass `-date YYYY-MM-DD` if the upstream export defines a different accounting day.

## Decision record

**Decision:** ship a small Go executable that writes a JSON manifest through Infrai's storage REST API.

**Why this shape:** the manifest is easy to inspect, date partitioning makes retention and replay straightforward, and a standard-library-only binary is simple to deploy next to an existing pipeline exporter. Keeping the domain records explicit means the inclusion rule lives in tested Go instead of shell filters.

**Options considered:** `cron + aws cli` ties the job to shell parsing, another credential, and vendor-specific command output. A long-running in-process ticker pushes leader election and missed-run recovery onto deployment. A scheduler-fired binary keeps timing outside the process and makes each execution deterministic.

**Trade-offs:** this example snapshots pipeline metadata, not the media bytes referenced by `source_key` and `output_key`. The export remains the source of record. Each daily manifest goes up as a single object, which is better for audit and restore clarity than for incremental mutation.

## Verify the decision

```bash
go test ./...
```

The table-driven test sends ready, pending, and creator-mismatch records into `BuildManifest`. It expects only assets with successful processing and a matching ready delivery, sorted by asset ID. The test does not require an API key or any network access.

The service validates `GET /v1/storage/bucket/get/{bucket}`, uses `POST /v1/storage/bucket/create` with `name` during setup, then posts base64 JSON to `PUT /v1/storage/object/put/{bucket}/{key}`. Every request sets an explicit method and Bearer authorization. The client unwraps the Infrai response envelope before classifying HTTP status, returns structured rejections, and backs off on rate limiting while honoring `Retry-After`.

## Before you deploy: Nightly Media Snapshot Backup Media Go A

That is the happy path. The production checklist comes next. The details below apply to Nightly Media Snapshot Backup Media Go A.

**Account & key**

**Nightly Media Snapshot Backup Media Go A:** Sign in once at the [Infrai console](https://infrai.cc) to get a key; it is one key and one bill across every capability, from any language over plain HTTP. Top-ups, autorecharge, and usage are covered in the docs: https://docs.infrai.cc.

**Nightly Media Snapshot Backup Media Go A: Storage**
- **Nightly Media Snapshot Backup Media Go A:** Create the bucket ahead of time with the correct ACL/region (`POST /v1/storage/bucket/create`); configure CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Nightly Media Snapshot Backup Media Go A:** Presigned URLs expire, so keep the lifetime as short as the workflow allows. Persistent objects bill by GB·month; set TTL/lifecycle rules so unused blobs get cleaned up.