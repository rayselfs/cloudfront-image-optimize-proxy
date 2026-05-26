# cf-image-optimize-proxy Helm Chart

Helm chart for deploying the image-optimize-proxy with an imgproxy sidecar on Kubernetes (EKS).

## Requirements

- Kubernetes ≥ 1.29
- Helm ≥ 3.8 (OCI registry support)
- IRSA configured for S3 read/write access

## Install

### From OCI registry (recommended)

The chart is published to GitHub Container Registry on every version tag.

```bash
helm install image-optimize-proxy \
  oci://ghcr.io/rayselfs/charts/cf-image-optimize-proxy \
  --version <chart-version> \
  --set image.repository=ghcr.io/rayselfs/image-optimize-proxy \
  --set image.tag=<app-version> \
  --set config.cacheS3Bucket=my-image-cache-bucket \
  --set config.cacheS3Region=us-east-1 \
  --set serviceAccount.roleArn=arn:aws:iam::<account-id>:role/<role-name>
```

### From local source

```bash
helm install image-optimize-proxy ./charts/cf-image-optimize-proxy \
  --set image.repository=ghcr.io/rayselfs/image-optimize-proxy \
  --set image.tag=<app-version> \
  --set config.cacheS3Bucket=my-image-cache-bucket \
  --set serviceAccount.roleArn=arn:aws:iam::<account-id>:role/<role-name>
```

## Upgrade

```bash
helm upgrade image-optimize-proxy \
  oci://ghcr.io/rayselfs/charts/cf-image-optimize-proxy \
  --version <new-chart-version> \
  --reuse-values \
  --set image.tag=<new-app-version>
```

## Values

| Key | Default | Description |
|-----|---------|-------------|
| `image.repository` | `image-optimize-proxy` | Proxy container image repository |
| `image.tag` | `v3.31.3` | Proxy container image tag |
| `imgproxy.image.repository` | `darthsim/imgproxy` | imgproxy sidecar image |
| `imgproxy.image.tag` | `v3.31.3` | imgproxy sidecar image tag |
| `replicaCount` | `2` | Number of pod replicas (overridden by HPA when enabled) |
| `config.cacheS3Bucket` | `""` | **(Required)** S3 bucket for cached images |
| `config.cacheS3Region` | `us-east-1` | AWS region of the S3 bucket |
| `config.maxWidth` | `1920` | Maximum allowed image width in pixels |
| `config.upstreamTimeout` | `30` | Upstream fetch timeout in seconds |
| `config.imgproxyTimeout` | `30` | imgproxy transform timeout in seconds |
| `config.allowedUpstreamGateways` | `""` | **(Required)** Comma-separated allowlist of upstream gateway URLs; server refuses to start if empty |
| `config.allowedSourceBuckets` | `""` | **(Required)** Comma-separated allowlist of source S3 buckets; server refuses to start if empty |
| `serviceAccount.roleArn` | `""` | IAM role ARN for IRSA (S3 access) |
| `hpa.enabled` | `true` | Enable HorizontalPodAutoscaler |
| `hpa.minReplicas` | `2` | HPA minimum replicas |
| `hpa.maxReplicas` | `10` | HPA maximum replicas |
| `ha.enabled` | `false` | Enable pod anti-affinity + topology spread constraints |
| `networkPolicy.enabled` | `false` | Enable NetworkPolicy |
| `podDisruptionBudget.enabled` | `true` | Enable PodDisruptionBudget |
| `serviceMonitor.enabled` | `false` | Enable Prometheus ServiceMonitor |
| `prometheusRule.enabled` | `false` | Enable PrometheusRule |
| `originVerify.secretName` | `""` | Secret name containing `CF_ORIGIN_SECRET` for CloudFront origin verification |
| `extraManifests` | `[]` | Additional raw Kubernetes manifests to render alongside the chart |

See [`values.yaml`](values.yaml) for the full reference including HPA behavior, lifecycle hooks, and security contexts.

---

## Publishing (GitHub Actions)

The chart is published automatically by [`.github/workflows/release.yml`](../../.github/workflows/release.yml).

### Trigger

The `helm` job runs **only on version tags** (`v*.*.*` pushed to the default branch). Pushing to
`main` without a tag builds and pushes the Docker image only — the chart is not published.

```
git tag v1.2.3
git push origin v1.2.3   # triggers both docker + helm jobs
```

### Versioning

| Field | Source | Example |
|-------|--------|---------|
| `chart version` | Git tag, `v` prefix stripped | `v1.2.3` → `1.2.3` |
| `appVersion` | Docker image version from the upstream `docker` job | `1.2.3` |

The `docker` job runs first; the `helm` job `needs: docker` so it receives the resolved image
version via job outputs and stamps it into the chart with `--app-version`.

### Published artifact

```
oci://ghcr.io/rayselfs/charts/cf-image-optimize-proxy:<chart-version>
```

The OCI registry is GHCR. Authentication uses the built-in `GITHUB_TOKEN` — no additional secrets
are required.

### Job steps

```yaml
# 1. Install Helm CLI (pinned to v3.17.0)
- uses: azure/setup-helm@v4
  with:
    version: v3.17.0

# 2. Authenticate to GHCR
- name: Login to GHCR
  run: echo "$GITHUB_TOKEN" | helm registry login ghcr.io --username "$GITHUB_ACTOR" --password-stdin

# 3. Resolve chart version from the Git tag
#    refs/tags/v1.2.3  →  1.2.3
- name: Resolve chart version
  run: VERSION="${GITHUB_REF_NAME#v}" && echo "version=${VERSION}" >> "$GITHUB_OUTPUT"

# 4. Package the chart, injecting version + appVersion
- name: Package Helm chart
  run: |
    helm package charts/cf-image-optimize-proxy \
      --version "${{ steps.version.outputs.version }}" \
      --app-version "${{ needs.docker.outputs.image-version }}"

# 5. Push the .tgz to GHCR OCI registry
- name: Push Helm chart to GHCR
  run: helm push cf-image-optimize-proxy-*.tgz oci://ghcr.io/rayselfs/charts
```

### Permissions

The workflow requires the following GitHub Actions permissions (already set in the job):

```yaml
permissions:
  contents: read
  packages: write   # required to push to GHCR
```

### Pulling the chart (private repo)

If the repository is private, authenticate before pulling:

```bash
echo "$CR_PAT" | helm registry login ghcr.io --username <github-username> --password-stdin
helm pull oci://ghcr.io/rayselfs/charts/cf-image-optimize-proxy --version <chart-version>
```

`CR_PAT` is a GitHub Personal Access Token with `read:packages` scope.
