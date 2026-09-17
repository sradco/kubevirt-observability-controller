# kubevirt-observability-controller

A Kubernetes controller that externalizes KubeVirt monitoring, metrics,
recording rules, and alerts, into a standalone operator, decoupled from the main
KubeVirt deployment.

## Overview

The controller watches KubeVirt resources (VirtualMachine,
VirtualMachineInstance, migrations, snapshots, restores, exports, clones,
pools, and instance types) and exposes Prometheus metrics about them. It also
reconciles Prometheus Operator resources (PrometheusRule, ServiceMonitor) so
Prometheus can scrape those metrics and evaluate alerting/recording rules.

## Prerequisites

- Kubernetes cluster (v1.26+) with KubeVirt installed
- `kubectl` configured to access the cluster
- Container build tool (`docker` or `podman`)
- prometheus-operator CRDs installed (for PrometheusRule and ServiceMonitor)

## Quick Start

Build and push the image:

```sh
IMG=<your-registry>/virt-observability-controller:latest
make docker-build docker-push IMG=$IMG
```

Deploy to the cluster:

```sh
make deploy IMG=$IMG
```

For detailed deployment options, configuration flags, and verification steps, see [docs/deployment.md](docs/deployment.md).

## Documentation

- [Deployment Guide](docs/deployment.md) — build, deploy, configure, and verify
- [Metrics Reference](docs/metrics.md) — full list of exposed metrics and recording rules
- [Running E2E Tests](docs/running-e2e-tests.md) — how to run end-to-end tests with kubevirtci

## Development

### Build

```sh
make build
```

### Run Tests

```sh
make test        # unit tests
make test-e2e    # end-to-end tests (requires kubevirtci cluster)
```

### Lint

```sh
make lint
make lint-fix
```

### Generate

```sh
make manifests   # regenerate RBAC manifests
make generate    # regenerate deepcopy and metrics docs
```

## Uninstall

If deployed with kustomize:

```sh
make undeploy
```

If deployed with a static manifest:

```sh
kubectl delete -f dist/install.yaml
```

## Contributing

Run `make help` for the full list of available `make` targets.

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
