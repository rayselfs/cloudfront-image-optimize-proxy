# Phase 1 Security Implementation Plan

**Scope**: X-Origin-Verify middleware · AllowedUpstreamGateways allowlist · Presigned URL log removal  
**Target files**: 7 Go files + 3 Helm files  
**Risk**: Low — all changes are additive or removal; no core transform logic touched  

---

## Context

```
CloudFront → NLB → proxy(:8080) → resolver → S3 presign / upstream gateway
```

Three layered defences being added:
1. **X-Origin-Verify** — ensures only CloudFront can reach the proxy (blocks forged X-Img-* headers from external actors)
2. **AllowedUpstreamGateways** — ensures even if CloudFront is misconfigured, only known gateways are used (mirrors imgproxy `IMGPROXY_ALLOWED_SOURCES`)
3. **Presigned URL log removal** — presign tokens must never appear in log sinks

---

## Invariants (MUST NOT break)

- `/health` and `/ready` endpoints must respond **without** X-Origin-Verify — Kubernetes liveness/readiness probes do not send the secret
- Cache key format `{host}/{path}/{width}_{format}_{quality}` must remain unchanged
- Pass-through for no-transform requests must remain unchanged
- If `CF_ORIGIN_SECRET` env var is **empty**, middleware MUST pass all requests (dev/local compatibility)
- If `ALLOWED_UPSTREAM_GATEWAYS` is **empty**, allowlist check is SKIPPED (opt-in, not breaking)

---

## File Change Map

```
internal/
  config/config.go              ← add OriginSecrets, AllowedUpstreamGateways
  middleware/origin_verify.go   ← NEW: X-Origin-Verify middleware
  middleware/origin_verify_test.go ← NEW: unit tests
  upstream/resolver.go          ← add allowedGateways field + validation
  upstream/resolver_test.go     ← add allowlist rejection test cases
  handler/handler.go            ← remove "source_url" from slog.Error line 141
  config/config_test.go         ← add tests for new fields
cmd/server/main.go              ← wire middleware + pass allowedGateways to resolver
charts/image-optimize-proxy/
  values.yaml                   ← add originVerify + allowedUpstreamGateways
  templates/configmap.yaml      ← add ALLOWED_UPSTREAM_GATEWAYS
  templates/deployment.yaml     ← add optional secretRef for CF_ORIGIN_SECRET
```

---

## Step-by-Step Implementation

### Step 1 — 1-C: Remove presigned URL from log (5 min, zero risk)

**File**: `internal/handler/handler.go`  
**Line**: 141

Change:
```go
// BEFORE
slog.Error("handler: transform", "source_url", sourceURL, "error", err)

// AFTER
slog.Error("handler: transform failed, using original",
    "error", err,
    "path", r.URL.Path,
    "cache_key", key,
)
```

**Why**: `sourceURL` for S3 sources is a presigned URL containing AWS credentials in query params. Writing it to logs leaks IAM-signed tokens to any log sink with access.

**Verification**: `grep -n "source_url" internal/handler/handler.go` → must return zero results.

---

### Step 2 — 1-A: Config — add OriginSecrets

**File**: `internal/config/config.go`

Add to `Config` struct:
```go
// OriginSecrets holds one or more valid X-Origin-Verify values.
// Set via CF_ORIGIN_SECRET env var (comma-separated for dual-secret rotation).
// If empty, origin verification is disabled (dev/local mode).
OriginSecrets []string
```

Add to `Load()` after existing fields:
```go
if raw := strings.TrimSpace(os.Getenv("CF_ORIGIN_SECRET")); raw != "" {
    parts := strings.Split(raw, ",")
    for _, p := range parts {
        if s := strings.TrimSpace(p); s != "" {
            cfg.OriginSecrets = append(cfg.OriginSecrets, s)
        }
    }
}
```

**Dual-secret rotation support**: `CF_ORIGIN_SECRET=new-secret,old-secret` — both accepted simultaneously. During rotation: update CloudFront header first → wait ~5 min for propagation → remove old secret from env.

---

### Step 3 — 1-B: Config — add AllowedUpstreamGateways

**File**: `internal/config/config.go`

Add to `Config` struct:
```go
// AllowedUpstreamGateways is the allowlist of permitted X-Img-Upstream-Gateway values.
// Set via ALLOWED_UPSTREAM_GATEWAYS env var (comma-separated hostnames, no scheme).
// If empty, all gateways are permitted (backward compatible).
AllowedUpstreamGateways []string
```

Add to `Load()`:
```go
if raw := strings.TrimSpace(os.Getenv("ALLOWED_UPSTREAM_GATEWAYS")); raw != "" {
    parts := strings.Split(raw, ",")
    for _, p := range parts {
        if s := strings.TrimSpace(p); s != "" {
            cfg.AllowedUpstreamGateways = append(cfg.AllowedUpstreamGateways, s)
        }
    }
}
```

---

### Step 4 — 1-A: New middleware — origin_verify.go

**File**: `internal/middleware/origin_verify.go` (NEW)

```go
package middleware

import (
    "log/slog"
    "net/http"
)

// exemptPaths are paths that bypass X-Origin-Verify (Kubernetes probes).
var exemptPaths = map[string]bool{
    "/health": true,
    "/ready":  true,
}

// CloudFrontVerify returns a middleware that validates the X-Origin-Verify header.
// If secrets is empty, all requests are allowed (dev/local mode).
// Exempt paths (/health, /ready) always bypass the check.
func CloudFrontVerify(secrets []string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Always allow probe endpoints.
            if exemptPaths[r.URL.Path] {
                next.ServeHTTP(w, r)
                return
            }

            // If no secrets configured, bypass check (local dev).
            if len(secrets) == 0 {
                next.ServeHTTP(w, r)
                return
            }

            token := r.Header.Get("X-Origin-Verify")
            for _, s := range secrets {
                if token == s {
                    next.ServeHTTP(w, r)
                    return
                }
            }

            slog.Warn("origin verify: unauthorized access attempt",
                "client_ip", r.RemoteAddr,
                "path", r.URL.Path,
                "method", r.Method,
            )
            http.Error(w, "Forbidden", http.StatusForbidden)
        })
    }
}
```

---

### Step 5 — 1-B: Resolver — add allowedGateways field + validation

**File**: `internal/upstream/resolver.go`

Update `DefaultResolver` struct:
```go
type DefaultResolver struct {
    httpClient      *http.Client
    allowedGateways []string // nil = allow all; populated = strict allowlist
}
```

Update `NewResolver` signature:
```go
func NewResolver(timeout time.Duration, allowedGateways []string) *DefaultResolver {
    return &DefaultResolver{
        httpClient:      &http.Client{Timeout: timeout},
        allowedGateways: allowedGateways,
    }
}
```

Update `Resolve()` — add validation after reading gateway header (after existing empty check):
```go
gateway := strings.TrimPrefix(r.Header.Get("X-Img-Upstream-Gateway"), "http://")
if gateway == "" {
    return "", nil, fmt.Errorf("X-Img-Upstream-Gateway header is required")
}

// Allowlist check — skipped if allowedGateways is empty.
if len(d.allowedGateways) > 0 {
    allowed := false
    for _, g := range d.allowedGateways {
        if gateway == g {
            allowed = true
            break
        }
    }
    if !allowed {
        return "", nil, fmt.Errorf("upstream gateway not in allowlist: %s", gateway)
    }
}
```

**Note**: Comparison is exact string match on hostname (no scheme). The existing `strings.TrimPrefix(..., "http://")` already strips the scheme before comparison.

---

### Step 6 — 1-A + 1-B: Wire in main.go

**File**: `cmd/server/main.go`

Update resolver construction (line 41):
```go
// BEFORE
resolver := upstream.NewResolver(cfg.UpstreamTimeout)

// AFTER
resolver := upstream.NewResolver(cfg.UpstreamTimeout, cfg.AllowedUpstreamGateways)
```

Update server handler (line 77):
```go
// BEFORE
Handler: middleware.Logging(mux),

// AFTER
Handler: middleware.Logging(middleware.CloudFrontVerify(cfg.OriginSecrets)(mux)),
```

**Middleware order**: `Logging` outermost → logs ALL requests including 403s → `CloudFrontVerify` → `mux`. This ensures access logs capture rejected attempts with their status code.

---

### Step 7 — Helm: values.yaml

**File**: `charts/image-optimize-proxy/values.yaml`

Add to `config:` section:
```yaml
config:
  # ... existing fields ...
  # Comma-separated gateway hostnames (no scheme) permitted via X-Img-Upstream-Gateway.
  # Leave empty to allow all gateways (not recommended for production).
  allowedUpstreamGateways: ""

# X-Origin-Verify secret configuration.
# The secret value is read from CF_ORIGIN_SECRET env var.
# Reference an existing Kubernetes Secret — do NOT store the secret value in values.yaml.
originVerify:
  # Name of the Kubernetes Secret containing CF_ORIGIN_SECRET.
  # Secret must have key: CF_ORIGIN_SECRET
  # Leave empty to disable origin verification (local dev only).
  secretName: ""
```

---

### Step 8 — Helm: configmap.yaml

**File**: `charts/image-optimize-proxy/templates/configmap.yaml`

Add:
```yaml
{{- if .Values.config.allowedUpstreamGateways }}
ALLOWED_UPSTREAM_GATEWAYS: {{ .Values.config.allowedUpstreamGateways | quote }}
{{- end }}
```

---

### Step 9 — Helm: deployment.yaml

**File**: `charts/image-optimize-proxy/templates/deployment.yaml`

After the existing `envFrom: - configMapRef:` block, add:
```yaml
          envFrom:
            - configMapRef:
                name: {{ include "image-optimize-proxy.fullname" . }}
            {{- if .Values.originVerify.secretName }}
            - secretRef:
                name: {{ .Values.originVerify.secretName }}
            {{- end }}
```

**Kubernetes Secret format** (created externally by Terraform/ops):
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: image-optimize-proxy-origin-verify
type: Opaque
stringData:
  CF_ORIGIN_SECRET: "your-secret-uuid-here"
```

---

## Tests to Write

### `internal/middleware/origin_verify_test.go` (NEW)

| Test case | Input | Expected |
|-----------|-------|----------|
| `TestCloudFrontVerify_NoSecrets` | secrets=[], any request | 200 (bypass) |
| `TestCloudFrontVerify_ValidToken` | secrets=["abc"], header="abc" | 200 |
| `TestCloudFrontVerify_WrongToken` | secrets=["abc"], header="xyz" | 403 |
| `TestCloudFrontVerify_MissingToken` | secrets=["abc"], no header | 403 |
| `TestCloudFrontVerify_DualSecret` | secrets=["new","old"], header="old" | 200 |
| `TestCloudFrontVerify_HealthExempt` | secrets=["abc"], path=/health, no header | 200 |
| `TestCloudFrontVerify_ReadyExempt` | secrets=["abc"], path=/ready, no header | 200 |
| `TestCloudFrontVerify_MetricsNotExempt` | secrets=["abc"], path=/metrics, no header | 403 |

### `internal/upstream/resolver_test.go` (ADD cases)

| Test case | Input | Expected |
|-----------|-------|----------|
| `TestResolve_AllowedGateway` | allowedGateways=["allowed.host"], header="allowed.host" | no error |
| `TestResolve_BlockedGateway` | allowedGateways=["allowed.host"], header="evil.host" | error containing "not in allowlist" |
| `TestResolve_EmptyAllowlist` | allowedGateways=[], header="any.host" | no error (bypass) |

### `internal/config/config_test.go` (ADD cases)

| Test case | Env | Expected |
|-----------|-----|----------|
| `TestLoad_OriginSecretsSingle` | CF_ORIGIN_SECRET=abc | OriginSecrets=["abc"] |
| `TestLoad_OriginSecretsMultiple` | CF_ORIGIN_SECRET=new,old | OriginSecrets=["new","old"] |
| `TestLoad_OriginSecretsEmpty` | CF_ORIGIN_SECRET="" | OriginSecrets=nil |
| `TestLoad_AllowedGateways` | ALLOWED_UPSTREAM_GATEWAYS=a.com,b.com | AllowedUpstreamGateways=["a.com","b.com"] |
| `TestLoad_AllowedGatewaysEmpty` | unset | AllowedUpstreamGateways=nil |

---

## Verification Checklist

After all steps complete, run in order:

```bash
# 1. No presigned URL in logs
grep -n "source_url" internal/handler/handler.go
# Expected: no output

# 2. Build succeeds
make build
# Expected: exit 0, binary at bin/image-optimize-proxy

# 3. All tests pass with race detector
make test
# Expected: all PASS, no races

# 4. LSP clean on changed files
# Run lsp_diagnostics on:
#   internal/config/config.go
#   internal/middleware/origin_verify.go
#   internal/upstream/resolver.go
#   internal/handler/handler.go
#   cmd/server/main.go
# Expected: zero errors

# 5. Lint
make lint
# Expected: exit 0
```

---

## Operational Notes

### Secret Rotation Procedure (zero-downtime)

```
1. Generate new secret:  uuidgen | tr -d '\n'
2. Update CF_ORIGIN_SECRET in Kubernetes Secret:  "new-secret,old-secret"
3. Rolling restart pods to pick up new env
4. Update CloudFront origin custom header to new-secret
5. Wait for CloudFront propagation (~5 min, verify with curl)
6. Remove old-secret from CF_ORIGIN_SECRET:  "new-secret"
7. Rolling restart pods again
```

### Terraform State Security

The CloudFront origin custom header value appears in Terraform state.
Ensure:
- S3 backend with `encrypt = true` and KMS key
- `sensitive = true` on the header output in Terraform
- State access limited via IAM to CI/CD role only

### Local Development

With `CF_ORIGIN_SECRET` unset, the middleware passes all requests — no local config changes needed.
With `ALLOWED_UPSTREAM_GATEWAYS` unset, all gateways are permitted — no local config changes needed.
