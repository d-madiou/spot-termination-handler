
```markdown
# Spot Termination Handler

![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)
![Status](https://img.shields.io/badge/status-in%20development-yellow.svg)

A lightweight, open source Kubernetes DaemonSet that watches for AWS EC2 Spot
Instance termination notices and gracefully drains the node before AWS reclaims it.

---

## How It Works

- Polls the EC2 instance metadata endpoint every 5 seconds
- On termination notice → cordons the node (no new pods scheduled)
- Evicts all running pods gracefully within the 2-minute window
- Optionally notifies a webhook (Slack, PagerDuty) when termination is detected

```
Every 5s: GET 169.254.169.254/latest/meta-data/spot/termination-time
              │
         404 → safe, keep polling
         200 → cordon node → evict pods → node terminates cleanly
```

---

## Project Status

This project is under active development.

| Milestone       | Status         |
|-----------------|----------------|
| v0.1.0 — MVP    | 🔨 In Progress |
| v0.2.0 — Helm   | ⬜ Planned     |
| v0.3.0 — Metrics| ⬜ Planned     |
| v1.0.0 — Stable | ⬜ Planned     |

### Completed

- [x] Project scaffold and repository structure
- [x] Environment-based configuration loader
- [x] EC2 metadata poller with context-aware polling
- [x] Node cordon and pod eviction (drain logic)

### In Progress

- [ ] Kubernetes client wrapper (`internal/k8s`)
- [ ] Mock metadata server for local testing (`mock/`)
- [ ] CI pipeline (`github/workflows/ci.yaml`)
- [ ] Main entrypoint (`cmd/handler/main.go`)
- [ ] Helm chart (`deploy/helm/`)

---

## Repository Structure

```
spot-termination-handler/
├── cmd/handler/          # entrypoint
├── internal/
│   ├── config/           # environment-based configuration ✅
│   ├── metadata/         # EC2 termination poller ✅
│   ├── drain/            # node cordon and pod eviction ✅
│   └── k8s/              # kubernetes client wrapper
├── pkg/events/           # webhook/notification interface
├── mock/                 # fake metadata server for local testing
├── deploy/
│   ├── helm/             # helm chart
│   └── kustomize/        # kustomize overlays
├── test/
│   ├── e2e/              # end-to-end tests
│   ├── integration/      # integration tests
│   └── fixtures/         # kind cluster config
├── docs/                 # architecture and guides
└── scripts/              # local dev scripts
```

---

## Local Development

### Prerequisites

```bash
go version      # 1.21+
docker version  # 20+
kubectl version # 1.28+
kind version    # 0.20+
helm version    # 3+
```

### Run Tests

```bash
# Unit tests
go test ./internal/... -v

# Specific package
go test ./internal/config/... -v
go test ./internal/metadata/... -v
go test ./internal/drain/... -v
```

---

## Configuration

All configuration is via environment variables:

| Variable                 | Default                    | Required | Description                        |
|--------------------------|----------------------------|----------|------------------------------------|
| `NODE_NAME`              | —                          | ✅ Yes   | Kubernetes node name               |
| `METADATA_URL`           | `http://169.254.169.254`   | No       | Override for local testing         |
| `POLL_INTERVAL_SECONDS`  | `5`                        | No       | How often to poll metadata         |
| `DRAIN_TIMEOUT_SECONDS`  | `120`                      | No       | Hard deadline for drain operation  |
| `GRACE_PERIOD_SECONDS`   | `90`                       | No       | Per-pod termination grace period   |
| `LOG_LEVEL`              | `info`                     | No       | debug, info, warn, error           |
| `WEBHOOK_URL`            | —                          | No       | Slack or PagerDuty webhook         |

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before
opening a pull request.

Branch strategy:
```
feature/* → PR → develop → PR → main (releases only)
```

---

## License

Apache 2.0 — see [LICENSE](LICENSE) for details.
```
