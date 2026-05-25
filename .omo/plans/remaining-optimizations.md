# Remaining Optimization Plan

## TL;DR
> **Summary**: Implement all remaining non-excluded optimization issues from the post-fix audit. Explicitly exclude the two accepted `r.Host` design decisions.
> **Deliverables**: Security hardening, performance/resilience refactors, Prometheus/OTel observability, Helm ops resources, and test coverage gaps.
> **Effort**: XL
> **Parallel**: YES - 7 waves
> **Critical Path**: T1 → T13 → T12 → T16 → T17 → T20 → Final Verification

## Context
### Original Request
Create a plan for all remaining issues except these two confirmed non-fixes:
- Do not fix forwarding `r.Host` to upstream gateway; it is required for Istio virtual-host routing.
- Do not fix including `r.Host` in cache key; it is required for multi-domain cache namespace isolation.

### Interview Summary
- User explicitly asked to exclude those two `r.Host` items from the remaining optimization plan.
- Those design decisions are documented in `internal/upstream/resolver.go` and `internal/cache/s3.go`.

### Metis Review (gaps addressed)
- Use explicit opt-ins: `ALLOW_ALL_UPSTREAM_GATEWAYS=true`, `ALLOW_ALL_SOURCE_BUCKETS=true`.
- Multipart threshold: 5 MiB, configurable via `S3_MULTIPART_THRESHOLD_BYTES`.
- OTel exporter: OTLP/HTTP, disabled unless `OTEL_EXPORTER_OTLP_ENDPOINT` is set.
- Coalescer cancellation item is metrics/design-note only; do not implement complex all-callers-cancel semantics.
- Security headers exclude CSP; CSP remains CloudFront responsibility.

## Work Objectives
### Core Objective
Safely implement the 28 remaining non-excluded security, resilience, observability, operations, and test-coverage optimizations without breaking production cache key invariants or CloudFront/Istio routing contracts.

### Deliverables
- Go source changes under `cmd/server` and `internal/*`.
- Helm chart updates under `charts/image-optimize-proxy/*`.
- Unit/integration tests for every behavior change.
- Evidence files under `.omo/evidence/`.

### Definition of Done
- `make test` passes.
- `make build` passes.
- `make lint` passes.
- `helm template image-optimize-proxy ./charts/image-optimize-proxy --set config.cacheS3Bucket=test-cache --set config.allowedUpstreamGateways=istio-ingressgateway.istio-system.svc.cluster.local --set config.allowedSourceBuckets=source-bucket` renders successfully.
- Final verification agents F1-F4 produce agent-executed evidence and approve. User approval is a post-verification completion gate, not a verification step.

### Must Have
- Production fail-fast when allowlists are empty unless explicit allow-all env is set.
- `sourceURL` passed to imgproxy must be URL-path escaped.
- Server must set `ReadHeaderTimeout` and `MaxHeaderBytes`.
- Metrics must use Prometheus client library with bounded labels.
- No presigned S3 URL logging.

### Must NOT Have
- Do NOT change cache key format: `{host}/{uri-path}/{imwidth}_{f}_{q}`.
- Do NOT remove `r.Host` forwarding to upstream gateway.
- Do NOT add CSP in proxy.
- Do NOT add full path/cache key/source URL as Prometheus labels.
- Do NOT modify NLB annotations.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: tests-after, with TDD for simple edge-case tests.
- QA policy: every task includes happy + failure/edge scenario.
- Evidence: `.omo/evidence/task-{N}-{slug}.{ext}`.

## Execution Strategy
### Parallel Execution Waves
Wave 1: Security/server safety — T1-T7.
Wave 2: Transport/S3 foundations — T8-T10, T13.
Wave 3: Handler performance — T11, T12, T14, T15.
Wave 4: Metrics foundation — T16-T18.
Wave 5: Tracing/Ops — T19-T22, T28.
Wave 6: Test coverage gaps — T23-T27.
Wave 7: Stabilization and final verification.

### Dependency Matrix
- T1 blocks production safety rollout.
- T13 blocks T12 streaming cache work.
- T16 blocks T17, T18, T20.
- T17 and T18 block T20 alert expressions.
- T17 blocks T22 custom-metrics HPA decisions.
- T8/T9 should precede T11/T12 to avoid client constructor churn.

### Agent Dispatch Summary
- Waves 1-2: `unspecified-high`, `quick`.
- Wave 3: `deep`, `unspecified-high`.
- Waves 4-5: `unspecified-high`, `deep`.
- Wave 6: `quick`, `unspecified-high`.

## TODOs
> Implementation + Test = ONE task. Never separate.

- [ ] 1. Production allowlist fail-fast with explicit allow-all opt-ins
  **What to do**: Add strict boolean envs `ALLOW_ALL_UPSTREAM_GATEWAYS` and `ALLOW_ALL_SOURCE_BUCKETS` in `internal/config/config.go`. In `cmd/server/main.go`, exit non-zero when each allowlist is empty and its opt-in is not true. Add Helm values/configmap keys with defaults false/empty.
  **Must NOT do**: Do not remove dev/local capability; explicit opt-in must still allow local permissive mode.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 1 | Blocks: production rollout | Blocked By: none.
  **References**: `internal/config/config.go:65-78`, `cmd/server/main.go:34-43`, `charts/image-optimize-proxy/templates/configmap.yaml`, `charts/image-optimize-proxy/values.yaml`.
  **Acceptance Criteria**: `ALLOW_ALL_UPSTREAM_GATEWAYS=true ALLOW_ALL_SOURCE_BUCKETS=true CACHE_S3_BUCKET=x go test ./internal/config ./cmd/server` passes; config tests cover empty allowlists with false opt-ins returning startup validation error.
  **QA Scenarios**: Happy: run config test with allowlists set, expect no error and evidence `.omo/evidence/task-1-allowlist-pass.txt`. Edge: run config/startup validation with empty allowlist and opt-in false, expect non-zero validation error and evidence `.omo/evidence/task-1-allowlist-fail.txt`.
  **Commit**: YES | Message: `security(config): fail fast on empty source allowlists`

- [ ] 2. Escape source URL in imgproxy processing URL
  **What to do**: In `internal/imgproxy/client.go`, wrap `sourceURL` with `url.PathEscape` before embedding in `/plain/`. Add unit tests for spaces, `%`, query string, slash, and Unicode.
  **Must NOT do**: Do not switch to imgproxy signed URL mode or base64 unless tests prove `PathEscape` breaks current imgproxy contract.
  **Recommended Agent Profile**: Category `quick`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 1 | Blocks: none | Blocked By: none.
  **References**: `internal/imgproxy/client.go:38-44`, `internal/imgproxy/client_test.go`.
  **Acceptance Criteria**: `go test ./internal/imgproxy -run TestBuildURL -v` passes with new special-character test.
  **QA Scenarios**: Happy: source URL with `?X-Amz-Signature=a/b` encodes into one path segment. Edge: Unicode path encodes and test asserts no raw query separator after `/plain/`.
  **Commit**: YES | Message: `security(imgproxy): escape source URL in processing path`

- [ ] 3. Add HTTP server header limits
  **What to do**: Extract HTTP server construction into a testable helper in `cmd/server/main.go` (e.g. `newHTTPServer(addr string, handler http.Handler) *http.Server`). Set `ReadHeaderTimeout: 5 * time.Second` and `MaxHeaderBytes: 64 << 10` in that helper. Add `cmd/server/main_test.go` asserting those exact fields.
  **Must NOT do**: Do not change existing `ReadTimeout`, `WriteTimeout`, or `IdleTimeout` values.
  **Recommended Agent Profile**: Category `quick`; Skills [].
  **Parallelization**: Wave 1 | Blocks: none | Blocked By: none.
  **References**: `cmd/server/main.go:75-81`.
  **Acceptance Criteria**: `go test ./cmd/server -run TestNewHTTPServerLimits -v` asserts `ReadHeaderTimeout == 5*time.Second` and `MaxHeaderBytes == 64<<10`; `make build` passes.
  **QA Scenarios**: Happy: `newHTTPServer(":8080", mux)` returns server with existing read/write/idle timeouts unchanged. Edge: test fails if either new header limit is zero.
  **Commit**: YES | Message: `security(server): constrain request header reads`

- [ ] 4. Make origin verification multi-secret comparison all-scan
  **What to do**: In `internal/middleware/origin_verify.go`, compute candidate token hash once, compare against every configured secret hash, OR the constant-time compare result, then decide after the loop.
  **Must NOT do**: Do not log secret values or header token.
  **Recommended Agent Profile**: Category `quick`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 1 | Blocks: none | Blocked By: none.
  **References**: `internal/middleware/origin_verify.go`, `internal/middleware/origin_verify_test.go`.
  **Acceptance Criteria**: `go test ./internal/middleware -run TestCloudFrontVerify -v` passes; dual-secret tests still pass.
  **QA Scenarios**: Happy: first and second secret both authorize. Edge: wrong token scans all secrets and returns 403.
  **Commit**: YES | Message: `security(origin): compare all origin secrets before returning`

- [ ] 5. Add security headers middleware without CSP
  **What to do**: Add `internal/middleware/security_headers.go` setting `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, and `Permissions-Policy: geolocation=(), microphone=(), camera=()`. Register it in `cmd/server/main.go` middleware chain.
  **Must NOT do**: Do not set CSP or HSTS in proxy.
  **Recommended Agent Profile**: Category `quick`; Skills [].
  **Parallelization**: Wave 1 | Blocks: none | Blocked By: none.
  **References**: `internal/middleware/logging.go`, `cmd/server/main.go`.
  **Acceptance Criteria**: `go test ./internal/middleware -run TestSecurityHeaders -v` passes.
  **QA Scenarios**: Happy: `/health` response contains configured headers. Edge: existing handler-set headers are not overwritten except target security headers.
  **Commit**: YES | Message: `security(headers): add default response hardening middleware`

- [ ] 6. Enforce pass-through size limit
  **What to do**: Update `Handler.passThrough` to honor `MaxBodyBytes`: if upstream `Content-Length` exceeds limit, return `413 Request Entity Too Large`; otherwise copy through `io.LimitReader` and detect limit overflow. Add tests for under/over limit.
  **Must NOT do**: Do not buffer entire pass-through body in memory.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 1 | Blocks: none | Blocked By: none.
  **References**: `internal/handler/handler.go:83-105`, `internal/handler/handler_test.go`.
  **Acceptance Criteria**: `go test ./internal/handler -run TestPassThrough -v` passes with new over-limit test.
  **QA Scenarios**: Happy: small pass-through streams 200. Edge: body over limit returns 413 and evidence file captures status.
  **Commit**: YES | Message: `security(handler): enforce pass-through body limit`

- [ ] 7. Sanitize and validate Content-Type values
  **What to do**: Add helper in `internal/handler` or `internal/cache` to reject content types containing CR/LF and require transformed content types start with `image/`. Pass-through may allow upstream content type only after control-character validation.
  **Must NOT do**: Do not reject non-image pass-through content solely because it is non-image; existing bypass behavior must remain.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [].
  **Parallelization**: Wave 1 | Blocks: none | Blocked By: none.
  **References**: `internal/handler/handler.go:99-101`, `internal/cache/s3.go:77-84`.
  **Acceptance Criteria**: `go test ./internal/handler ./internal/cache -v` passes with tests for CR/LF rejection and transform non-image rejection.
  **QA Scenarios**: Happy: `image/webp` stored and served. Edge: `image/png\r\nX-Bad: 1` returns safe error/no injected header.
  **Commit**: YES | Message: `security(content): validate content type headers`

- [ ] 8. Introduce shared tuned HTTP transport
  **What to do**: Add a shared transport constructor (e.g. `internal/httpclient/transport.go` or `cmd/server` local helper) with `MaxIdleConns`, `MaxIdleConnsPerHost`, `IdleConnTimeout`, `TLSHandshakeTimeout`. Update `imgproxy.NewClient` and `upstream.NewResolver` to accept optional `*http.Client` or `http.RoundTripper` while preserving existing constructors via wrapper.
  **Must NOT do**: Do not change `ImgproxyTimeout` or `UpstreamTimeout` semantics.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [].
  **Parallelization**: Wave 2 | Blocks: T11, T12 | Blocked By: none.
  **References**: `internal/imgproxy/client.go:29-35`, `internal/upstream/resolver.go:59-65`, `cmd/server/main.go`.
  **Acceptance Criteria**: `make test` passes; tests prove existing constructor behavior remains.
  **QA Scenarios**: Happy: both clients use provided transport in unit test. Edge: nil transport preserves previous behavior.
  **Commit**: YES | Message: `perf(http): use shared tuned transport for downstream clients`

- [ ] 9. Initialize S3 presigner at startup and inject resolver dependency
  **What to do**: Refactor `upstream.DefaultResolver` to accept a `s3Presigner` during construction or via new constructor; create presigner in `cmd/server/main.go` during startup using existing AWS config. Keep test seam for mock presigner.
  **Must NOT do**: Do not reintroduce request-context cancellation into presigner construction.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [].
  **Parallelization**: Wave 2 | Blocks: none | Blocked By: T8 preferred.
  **References**: `internal/upstream/resolver.go:29-35`, `cmd/server/main.go:34-43`.
  **Acceptance Criteria**: `go test ./internal/upstream ./cmd/server -v` passes; startup fails if presigner cannot initialize.
  **QA Scenarios**: Happy: mock presigner injected and S3 resolve works. Edge: presigner construction error produces startup validation error.
  **Commit**: YES | Message: `perf(upstream): initialize S3 presigner at startup`

- [ ] 10. Make async cache worker pool size and put timeout configurable
  **What to do**: Replace `maxConcurrentPuts = 32` constant with config `ASYNC_CACHE_PUT_CONCURRENCY`, default 32. Replace the hardcoded `30*time.Second` timeout in `cmd/server/main.go:52` (`WrapAsyncPut(s3Cache, 30*time.Second)`) with config `ASYNC_CACHE_PUT_TIMEOUT_SECONDS`, default 30. Wire both through `internal/config/config.go`, `cmd/server/main.go`, and Helm configmap/values. Validate both as positive integers.
  **Must NOT do**: Do not create an unbounded queue. Do not remove the timeout entirely (zero/negative must be rejected).
  **Recommended Agent Profile**: Category `quick`; Skills [].
  **Parallelization**: Wave 2 | Blocks: T14, T18 | Blocked By: none.
  **References**: `internal/cache/async.go:11-28`, `internal/config/config.go`, `cmd/server/main.go:52`, `charts/image-optimize-proxy/values.yaml`.
  **Acceptance Criteria**: `go test ./internal/cache ./internal/config -v` passes with custom concurrency and timeout tests; `make build` passes.
  **QA Scenarios**: Happy: defaults are 32 concurrency and 30s timeout. Edge: zero/negative for either env returns config error.
  **Commit**: YES | Message: `perf(cache): make async put concurrency and timeout configurable`

- [ ] 11. Defer original upstream fetch until transform fallback is needed
  **What to do**: Extend `upstream.Resolver` to return a `headFunc func() (contentType string, err error)` in addition to `sourceURL` and `fetchFunc`. For gateway sources, `headFunc` sends HTTP HEAD to the same `sourceURL` and intentionally forwards `r.Host` for Istio virtual-host routing. For S3 sources, extend the presigner seam with `PresignHeadObject` and HEAD the presigned HEAD URL. Handler flow becomes: cache miss → resolve → `headFunc`; if HEAD returns a non-image content type, call `fetchFunc` and bypass transform; if HEAD returns image or empty/unsupported (405 or network error), attempt transform; if transform fails, call `fetchFunc` exactly once for fallback.
  **Must NOT do**: Do not remove imgproxy fallback to original image.
  **Recommended Agent Profile**: Category `deep`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 3 | Blocks: T12 | Blocked By: T8 preferred.
  **References**: `internal/handler/handler.go:108-166`, `internal/handler/handler_test.go`.
  **Acceptance Criteria**: `go test ./internal/handler ./internal/upstream -run 'Test(CacheMissTransform|NonImage|Resolve.*Head)' -v` passes; successful image transform records zero `fetchFunc` calls and one `headFunc` call.
  **QA Scenarios**: Happy: HEAD reports `image/png`, transform succeeds, original body is never fetched. Edge: HEAD reports `text/html`, handler bypasses transform and fetches original once; HEAD 405 proceeds transform-first and falls back on transform error.
  **Commit**: YES | Message: `perf(handler): defer original fetch until fallback`

- [ ] 12. Stream transformed output through coalescer-safe temp-file cache fill
  **What to do**: Replace full `io.ReadAll(transformedBody)` path with a coalescer-safe temp-file cache-fill pipeline. Fixed implementation model: (1) `ServeHTTP` checks cache before `Coalescer.Do`; HIT streams directly via `io.Copy` from `Cache.Get`. (2) On miss, `Coalescer.Do` runs a leader function that transforms once, writes transformed body to a temp file via `os.CreateTemp("", "image-optimize-proxy-*")`, and enforces `MaxBodyBytes` with `io.LimitReader(MaxBodyBytes+1)`. (3) If size exceeds limit, leader closes/removes temp file and returns an error result; no cache write. (4) Add `cache.FileCache` with synchronous `PutFile(ctx, key, path, contentType) error`; `S3Cache.PutFile` opens the file, uploads via T13 multipart path, closes it, and removes it. (5) `AsyncPutCache.PutFile` MUST delegate synchronously to the inner `FileCache` so `PutFile` does not return until the object is readable from cache; existing `Put` remains async for non-file callers. (6) The leader returns only metadata `{kind: cache_filled, key, contentType, cacheStatus: MISS}` from `Coalescer.Do`, never a temp-file path. (7) Every caller, including the leader and all waiters, then calls `Cache.Get(ctx, key)` and streams its own reader to `ResponseWriter`. (8) Fallback/non-image bypass results may still return bounded in-memory `[]byte` using `readBody`, preserving current fallback semantics.
  **Must NOT do**: Do not return a temp-file path as a shared coalescer result. Do not make cache fill async for transformed miss responses, because waiters must be able to read the cached object immediately after `Coalescer.Do` returns.
  **Recommended Agent Profile**: Category `deep`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 3 | Blocks: large-memory risk resolution | Blocked By: T13, T11.
  **References**: `internal/handler/handler.go:55-166`, `internal/cache/async.go`, `internal/cache/s3.go`, `internal/coalesce/coalesce.go`.
  **Acceptance Criteria**: `go test ./internal/handler ./internal/cache -run 'Test.*TempFile|Test.*PutFile|TestCacheMissTransform|TestConcurrent' -race -v` passes; tests assert temp files are removed after success and after over-limit failure; a concurrent cache-miss test proves one transform, one `PutFile`, and N independent `Cache.Get` response streams.
  **QA Scenarios**: Happy: five concurrent MISS requests for same key produce one imgproxy transform, one synchronous `PutFile`, five successful cache-backed responses, and no temp file left on disk. Edge: transformed body exceeds `MaxBodyBytes`, all coalesced callers receive a 502/limit error, cache is not called, and temp file is removed.
  **Commit**: YES | Message: `perf(handler): stream transformed images through bounded storage`

- [ ] 13. Add multipart S3 uploader with configurable 5 MiB threshold
  **What to do**: Use `github.com/aws/aws-sdk-go-v2/feature/s3/manager.Uploader` as the exact multipart implementation. Extend `S3Cache` to hold both the existing S3 client and a `manager.Uploader` configured with `PartSize = S3_MULTIPART_THRESHOLD_BYTES` defaulting to `5 * 1024 * 1024`. For bodies below threshold, keep current `PutObject`; for file uploads at or above threshold, use `Uploader.Upload`. Add config + Helm values for `S3_MULTIPART_THRESHOLD_BYTES`.
  **Must NOT do**: Do not change cache key format or cache-control semantics.
  **Recommended Agent Profile**: Category `deep`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 2 | Blocks: T12 | Blocked By: none.
  **References**: `internal/cache/s3.go:76-86`, `internal/config/config.go`, `charts/image-optimize-proxy/values.yaml`.
  **Acceptance Criteria**: `go test ./internal/cache -run TestS3Cache -v` passes with mocked assertions: small file uses `PutObject`, large file uses `manager.Uploader` path, both preserve content type/cache-control.
  **QA Scenarios**: Happy: 1 MiB file uses current PutObject. Edge: 6 MiB file uses multipart uploader and removes temp file after upload.
  **Commit**: YES | Message: `perf(cache): support multipart uploads for large cached images`

- [ ] 14. Test async cache pool-full and timeout paths
  **What to do**: Add deterministic tests in `internal/cache/async_test.go` using a helper-created `AsyncPutCache` with small semaphore capacity. Cover dropped put path, timeout path, and `Wait()` returning.
  **Must NOT do**: Do not rely on sleeps longer than necessary; avoid flaky timing.
  **Recommended Agent Profile**: Category `quick`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 3 | Blocks: none | Blocked By: T13 preferred.
  **References**: `internal/cache/async.go`, `internal/cache/async_test.go`.
  **Acceptance Criteria**: `go test ./internal/cache -run TestWrapAsyncPut -race -v` passes.
  **QA Scenarios**: Happy: all queued puts finish. Edge: saturated pool drops extra put and increments metric/log path without deadlock.
  **Commit**: YES | Message: `test(cache): cover async put saturation and timeout`

- [ ] 15. Redact async dropped-key logs
  **What to do**: Replace full cache key logging in `AsyncPutCache.Put` drop/failure logs with deterministic short hash, e.g. `sha256(key)[:12]`, and keep no full path in logs.
  **Must NOT do**: Do not use hash as Prometheus label.
  **Recommended Agent Profile**: Category `quick`; Skills [].
  **Parallelization**: Wave 3 | Blocks: none | Blocked By: none.
  **References**: `internal/cache/async.go`.
  **Acceptance Criteria**: unit test or log-capture test proves raw key is absent and hash is present.
  **QA Scenarios**: Happy: log includes `key_hash`. Edge: path `/private/user/email.png` does not appear in log output.
  **Commit**: YES | Message: `security(cache): redact async cache keys in logs`

- [ ] 16. Migrate metrics to Prometheus client library
  **What to do**: Add `github.com/prometheus/client_golang/prometheus` and `promhttp`. Replace hand-written text handler with registered counters/gauges/histograms while keeping current metric names or documenting deliberate name changes. Expose Go/process collectors.
  **Must NOT do**: Do not add high-cardinality labels.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 4 | Blocks: T17, T20, T21, T27 | Blocked By: none.
  **References**: `internal/metrics/metrics.go`, `cmd/server/main.go:72`.
  **Acceptance Criteria**: `go test ./internal/metrics -v` passes and `/metrics` output contains Go runtime metrics.
  **QA Scenarios**: Happy: existing counter increments render. Edge: Reset works for tests without global collector conflicts.
  **Commit**: YES | Message: `feat(metrics): migrate to prometheus client collectors`

- [ ] 17. Add request and stage latency histograms with bounded labels
  **What to do**: Add histograms: `http_request_duration_seconds{method,status_class,cache_status}`, `imgproxy_duration_seconds{status_class}`, `s3_get_duration_seconds{outcome}`, `s3_put_duration_seconds{outcome}`, `upstream_fetch_duration_seconds{outcome}`. Instrument middleware, imgproxy client, cache, and resolver.
  **Must NOT do**: Do not label by full path, URL, cache key, or bucket name.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [].
  **Parallelization**: Wave 4 | Blocks: T20, T21, T23 | Blocked By: T16.
  **References**: `internal/middleware/logging.go`, `internal/imgproxy/client.go`, `internal/cache/s3.go`, `internal/upstream/resolver.go`.
  **Acceptance Criteria**: metrics tests assert histogram samples exist after mocked requests.
  **QA Scenarios**: Happy: 200 HIT request records duration. Edge: imgproxy 500 records status_class `5xx`.
  **Commit**: YES | Message: `feat(metrics): add request and downstream latency histograms`

- [ ] 18. Add async cache inflight/drop metrics and coalescer observability note
  **What to do**: Add `async_cache_put_inflight` gauge and `async_cache_put_dropped_total` counter. Optionally add `coalescer_shared_total` and `coalescer_cancelled_waiters_total` if feasible without interface churn. Document that underlying singleflight work intentionally continues after caller cancellation.
  **Must NOT do**: Do not implement all-waiters-cancel cancellation semantics.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [].
  **Parallelization**: Wave 4 | Blocks: none | Blocked By: T13, T16.
  **References**: `internal/cache/async.go`, `internal/coalesce/coalesce.go`, `internal/metrics/metrics.go`.
  **Acceptance Criteria**: async metrics tests verify inflight increments/decrements and drop counter increments.
  **QA Scenarios**: Happy: successful async put returns gauge to zero. Edge: saturated pool increments dropped counter.
  **Commit**: YES | Message: `feat(metrics): expose async cache and coalescer signals`

- [ ] 19. Add request correlation ID middleware and propagation
  **What to do**: Accept `X-Request-Id` if present, else generate a UUID-like random ID using stdlib crypto/rand. Store in context, set response header, include in slog fields, propagate to imgproxy/upstream requests.
  **Must NOT do**: Do not add a new dependency solely for UUID generation.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [].
  **Parallelization**: Wave 5 | Blocks: T22 tracing integration quality | Blocked By: none.
  **References**: `internal/middleware/logging.go`, `internal/imgproxy/client.go`, `internal/upstream/resolver.go`.
  **Acceptance Criteria**: middleware tests assert generated and forwarded IDs.
  **QA Scenarios**: Happy: request with `X-Request-Id=abc` logs/returns `abc`. Edge: missing header generates non-empty ID.
  **Commit**: YES | Message: `feat(observability): add request correlation IDs`

- [ ] 20. Add Helm observability resources: ServiceMonitor and PrometheusRule
  **What to do**: Add optional `templates/servicemonitor.yaml` gated by `serviceMonitor.enabled`, and optional `templates/prometheusrule.yaml` gated by `prometheusRule.enabled`. ServiceMonitor values include namespace, interval, scrapeTimeout, and labels. PrometheusRule alerts must cover high 5xx rate, high imgproxy error rate, cache put failures/drops, high miss ratio, and p95 latency using metric names from T16/T17.
  **Must NOT do**: Do not require Prometheus Operator CRDs when disabled. Do not reference metric names that T16/T17 do not define.
  **Recommended Agent Profile**: Category `quick`; Skills [].
  **Parallelization**: Wave 5 | Blocks: none | Blocked By: T16, T17, T18.
  **References**: `charts/image-optimize-proxy/templates/service.yaml`, `charts/image-optimize-proxy/values.yaml`, metrics names from T16/T17.
  **Acceptance Criteria**: `helm template ... --set serviceMonitor.enabled=true --set prometheusRule.enabled=true` renders both resources; disabled values render no Prometheus Operator CRDs.
  **QA Scenarios**: Happy: enabled ServiceMonitor includes `/metrics` and PrometheusRule includes all five alert families. Edge: disabled chart remains CRD-free.
  **Commit**: YES | Message: `feat(helm): add optional prometheus monitoring resources`

- [ ] 21. Add optional S3 readiness check
  **What to do**: Add config `S3_READINESS_CHECK_ENABLED` default false and optional timeout. In `/ready`, when enabled, perform lightweight S3 permission check using existing cache bucket client, e.g. `HeadBucket` or equivalent API seam; keep imgproxy check.
  **Must NOT do**: Do not make S3 readiness mandatory by default.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [].
  **Parallelization**: Wave 5 | Blocks: none | Blocked By: none.
  **References**: `cmd/server/main.go:54-71`, `internal/cache/s3.go`, Helm configmap.
  **Acceptance Criteria**: tests cover readiness true/false paths; `make test` passes.
  **QA Scenarios**: Happy: imgproxy+S3 healthy returns 200. Edge: S3 readiness enabled and fails returns 503.
  **Commit**: YES | Message: `feat(readiness): add optional S3 health check`

- [ ] 22. Add Helm PDB, NetworkPolicy, and HPA custom-metric hardening options
  **What to do**: Keep existing PDB default unless product owner changes it, but add clear production values examples. Add optional NetworkPolicy template. Add optional HPA custom metric values for request rate/latency once metrics exist.
  **Must NOT do**: Do not modify NLB annotations or force NetworkPolicy enabled by default.
  **Recommended Agent Profile**: Category `unspecified-high`; Skills [].
  **Parallelization**: Wave 5 | Blocks: none | Blocked By: T17.
  **References**: `charts/image-optimize-proxy/templates/hpa.yaml`, `pdb.yaml`, `values.yaml`.
  **Acceptance Criteria**: helm template renders with each option enabled and disabled.
  **QA Scenarios**: Happy: NetworkPolicy enabled renders ingress/egress rules. Edge: all hardening options disabled preserves current output.
  **Commit**: YES | Message: `feat(helm): add optional network and scaling hardening`

- [ ] 23. Add config validation edge-case tests
  **What to do**: Add table-driven tests for invalid/zero/negative/overflow env values: `MAX_WIDTH`, `MAX_BODY_BYTES`, `UPSTREAM_TIMEOUT`, `IMGPROXY_TIMEOUT`, `SHUTDOWN_TIMEOUT`, new async concurrency, and multipart threshold.
  **Must NOT do**: Do not loosen existing config validation.
  **Recommended Agent Profile**: Category `quick`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 6 | Blocks: none | Blocked By: T1/T10/T13 config additions.
  **References**: `internal/config/config.go`, `internal/config/config_test.go`.
  **Acceptance Criteria**: `go test ./internal/config -v` passes.
  **QA Scenarios**: Happy: valid env values parse. Edge: invalid/overflow values return descriptive errors.
  **Commit**: YES | Message: `test(config): cover invalid runtime settings`

- [ ] 24. Add cache key edge-case tests without changing format
  **What to do**: Add tests for `KeyFromRequest` with root path `/`, host with port, percent-encoded path, and spaces. Assert current format exactly to document invariant.
  **Must NOT do**: Do not change `KeyFromRequest` behavior.
  **Recommended Agent Profile**: Category `quick`; Skills [].
  **Parallelization**: Wave 6 | Blocks: none | Blocked By: none.
  **References**: `internal/cache/s3.go:31-36`, `internal/cache/s3_test.go`.
  **Acceptance Criteria**: `go test ./internal/cache -run TestKeyFromRequest -v` passes.
  **QA Scenarios**: Happy: normal path unchanged. Edge: `/` produces documented double slash key.
  **Commit**: YES | Message: `test(cache): document cache key edge cases`

- [ ] 25. Add resolver edge-case tests
  **What to do**: Add tests for scheme-less gateway header, presigner construction error, requestURI empty path, and context cancellation during retry backoff.
  **Must NOT do**: Do not remove existing allowlist behavior tests.
  **Recommended Agent Profile**: Category `quick`; Skills [].
  **Parallelization**: Wave 6 | Blocks: none | Blocked By: T9 may change presigner seam.
  **References**: `internal/upstream/resolver.go`, `internal/upstream/resolver_test.go`.
  **Acceptance Criteria**: `go test ./internal/upstream -v` passes.
  **QA Scenarios**: Happy: `X-Img-Upstream-Gateway=host:port` becomes `http://host:port/...`. Edge: presigner init error returns error.
  **Commit**: YES | Message: `test(upstream): cover resolver edge cases`

- [ ] 26. Add imgproxy fallback integration test
  **What to do**: Extend `internal/handler/integration_test.go` with imgproxy server returning 500 and upstream returning image body. Assert proxy returns original body, `X-Cache: MISS`, and cache does not store transformed error body.
  **Must NOT do**: Do not rely on real S3 or real imgproxy.
  **Recommended Agent Profile**: Category `quick`; Skills [].
  **Parallelization**: Wave 4 or 6 | Blocks: none | Blocked By: none.
  **References**: `internal/handler/integration_test.go`, `internal/handler/handler.go`.
  **Acceptance Criteria**: `go test ./internal/handler -run TestIntegration.*Fallback -v` passes.
  **QA Scenarios**: Happy: imgproxy 200 path still passes existing integration. Edge: imgproxy 500 returns original.
  **Commit**: YES | Message: `test(handler): cover imgproxy fallback integration`

- [ ] 27. Add ParseParams non-integer quality test
  **What to do**: Add `q=abc` case in `TestParseParams` expecting default quality 75.
  **Must NOT do**: Do not change existing query param normalization rules.
  **Recommended Agent Profile**: Category `quick`; Skills [].
  **Parallelization**: Wave 6 | Blocks: none | Blocked By: none.
  **References**: `internal/handler/params.go`, `internal/handler/handler_test.go`.
  **Acceptance Criteria**: `go test ./internal/handler -run TestParseParams -v` passes.
  **QA Scenarios**: Happy: `q=80` remains 80. Edge: `q=abc` returns 75.
  **Commit**: YES | Message: `test(handler): cover invalid quality fallback`

- [ ] 28. Add OpenTelemetry OTLP/HTTP tracing, disabled by default
  **What to do**: Add optional OTel initialization in `cmd/server/main.go` when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Instrument inbound HTTP, imgproxy call, upstream fetch, and S3 operations with spans and bounded attributes.
  **Must NOT do**: Do not require collector endpoint for normal startup. Do not put full URL/cache key/presigned URL in span attributes.
  **Recommended Agent Profile**: Category `deep`; Skills [`karpathy-guidelines`].
  **Parallelization**: Wave 5 | Blocks: none | Blocked By: T19 preferred.
  **References**: `cmd/server/main.go`, `internal/imgproxy/client.go`, `internal/upstream/resolver.go`, `internal/cache/s3.go`.
  **Acceptance Criteria**: `make test` passes with OTel disabled; tests verify no exporter initialization when env empty.
  **QA Scenarios**: Happy: env unset starts without tracing. Edge: env set to test endpoint initializes provider without panic.
  **Commit**: YES | Message: `feat(tracing): add optional OTLP HTTP instrumentation`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL verification is agent-executed and must APPROVE. Present consolidated results to user after verification; user approval is a completion gate, not a manual verification step.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Agent-Executed QA — unspecified-high
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Use multiple commits by wave or tightly-related subsystem.
- Suggested sequence:
  1. `security: harden startup validation and response handling`
  2. `perf: tune transport and cache upload flow`
  3. `refactor: stream handler transform pipeline`
  4. `feat: add prometheus metrics and tracing`
  5. `feat: add helm observability and ops resources`
  6. `test: cover remaining edge cases`

## Success Criteria
- All 28 non-excluded findings are represented by implementation/test tasks.
- The two excluded `r.Host` findings are not present as fix tasks.
- All commands in Definition of Done pass.
- No task requires manual verification.
