# Running E2E Tests Locally

## Prerequisites

- Docker (or Podman, set `CONTAINER_TOOL=podman`)
- Go 1.24+
- `kubectl`

## 1. Start the cluster

Bring up a local Kubernetes cluster with KubeVirt, Prometheus Operator, and the VMStatsCollector feature gate enabled:

```bash
hack/kubevirt.sh up
```

This clones the KubeVirt repo (if not already present), starts a kubevirtci cluster (`k8s-1.36`), and deploys KubeVirt.

If KubeVirt is already running and you just need to re-sync it:

```bash
hack/kubevirt.sh sync
```

## 2. Export KUBECONFIG

```bash
export KUBECONFIG=$(hack/kubevirt.sh kubeconfig)
```

## 3. Build and push the controller image

```bash
REGISTRY=$(hack/kubevirt.sh registry)
docker build -t "${REGISTRY}/virt-observability-controller:latest" .
docker push --tls-verify=false "${REGISTRY}/virt-observability-controller:latest"
```

## 4. Run all e2e tests

```bash
make test-e2e IMG=registry:5000/virt-observability-controller:latest
```

This runs both suites sequentially:
- `test/monitoring/rules/` — PrometheusRule reconciliation and allowlist filtering
- `test/monitoring/metrics/` — custom metrics, metrics allowlist, VMStats, and inventory metrics

## 5. Run specific tests

Use `-ginkgo.focus` with a regex to run only matching specs.

Run only the metrics allowlist tests:

```bash
IMG=registry:5000/virt-observability-controller:latest \
  go test ./test/monitoring/metrics/ -v -ginkgo.v -ginkgo.show-node-events \
  -ginkgo.focus="Metrics Allowlist" -timeout 30m
```

Run only the rules allowlist tests:

```bash
IMG=registry:5000/virt-observability-controller:latest \
  go test ./test/monitoring/rules/ -v -ginkgo.v -ginkgo.show-node-events \
  -ginkgo.focus="Allowlist Filtering" -timeout 30m
```

Run only the VMStats tests:

```bash
IMG=registry:5000/virt-observability-controller:latest \
  go test ./test/monitoring/metrics/ -v -ginkgo.v -ginkgo.show-node-events \
  -ginkgo.focus="VMStats Metrics" -timeout 30m
```

Run only the inventory metric tests:

```bash
IMG=registry:5000/virt-observability-controller:latest \
  go test ./test/monitoring/metrics/ -v -ginkgo.v -ginkgo.show-node-events \
  -ginkgo.focus="Inventory metrics" -timeout 30m
```

Run a single test by name:

```bash
IMG=registry:5000/virt-observability-controller:latest \
  go test ./test/monitoring/metrics/ -v -ginkgo.v -ginkgo.show-node-events \
  -ginkgo.focus="should expose kubevirt_vmi_info" -timeout 30m
```

## 6. Tear down the cluster

```bash
hack/kubevirt.sh down
```
