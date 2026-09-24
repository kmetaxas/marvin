# Marvin

Marvin is a Go agent that runs on infrastructure (Kubernetes, VMs, etc.) and executes troubleshooting tasks on behalf of an AI-driven Control Plane. It advertises capabilities such as DNS lookups, ICMP pings, Kubernetes API queries, Prometheus queries, Kafka diagnostics, and Linux system introspection — then returns structured results over a persistent gRPC stream.

The goal is to give an AI agent hands-on, safe access to production systems without requiring direct shell access or ad-hoc command execution.

---

## How it works

1. **Registration** — On startup, Marvin reads a YAML config, builds its capability registry, and opens a bi-directional gRPC stream to the Control Plane.
2. **Capability advertisement** — It sends its hostname, metadata (cluster, region, environment), and the list of capabilities it can execute.
3. **Heartbeat** — Periodic keepalive messages let the Control Plane know the agent is healthy.
4. **Execution** — The Control Plane sends `ExecuteCapability` requests; Marvin runs the matching task and returns a structured `CapabilityResult`.
5. **Dynamic updates** — The Control Plane can push configuration updates (e.g., new Prometheus endpoints) without restarting the agent.

---

## Architecture

The codebase is organised in three layers:

| Layer                   | Responsibility                                                                                                    | Examples                                                                |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| **Capability Registry** | Discovers which capabilities are supported, enabled, and configured. Drives re-registration when the set changes. | `internal/registry`                                                     |
| **Providers**           | Group related tasks by domain. Each provider reports its capabilities to the registry.                            | `network`, `kubernetes`, `prometheus`, `kafka`, `linux`, `azure`, `log` |
| **Tasks**               | The actual implementation of a capability. Pure Go, no external process calls.                                    | `network.dns.lookup`, `kubernetes.pod.list`, `prometheus.query.instant` |

**Design constraints**

- **No CGO** — Static binary, zero OS dependencies.
- **Pure Go** — All functionality implemented with native Go libraries (e.g., `gopacket`-style networking, `k8s.io/client-go`, `franz-go` for Kafka).
- **Structured results** — Every task returns JSON with `success`, `error`, `data`, and `timestamp` fields.
- **Guardrails** — Providers enforce limits (max query range, max returned series, etc.) to prevent accidental overload.

---

## Capabilities

Capabilities are namespaced strings like `domain.resource.action`. A sample of what Marvin supports:

- `network.dns.lookup` — Resolve DNS names.
- `network.icmp.echo_request` — Ping hosts.
- `network.tcp.connect` — Test TCP connectivity.
- `network.tls.inspect` — Inspect TLS certificates.
- `network.traceroute` — Perform traceroutes (Linux).
- `kubernetes.pod.list` / `kubernetes.pod.get` / `kubernetes.pod.logs`
- `kubernetes.deployment.list` / `kubernetes.deployment.get`
- `kubernetes.node.list` / `kubernetes.node.get`
- `kubernetes.event.list`
- `kubernetes.configmap.list` / `kubernetes.configmap.get`
- `prometheus.query.instant` / `prometheus.query.range`
- `prometheus.series.query`
- `kafka.cluster.describe`
- `kafka.topic.list` / `kafka.topic.describe`
- `kafka.consumer_group.list`
- `kafka.broker.network.connectivity`
- `linux.system.info`
- `linux.file.read`
- `linux.process.list`
- `log.graylog.search`
- `azure.network.nsg.list`
- `os.process.list`
- `os.file.read`

(For the full list, see `internal/provider/*/provider.go` or run `go run ./cmd/provider-schema`.)

---

## Quick start

### Prerequisites

- Go 1.25+
- A Control Plane gRPC endpoint (Vogon)

### Build

```bash
go build -o marvin ./cmd/marvin
```

### Configure

Create a config file (default path: `/etc/marvin/config.yaml`):

```yaml
agent_id: "marvin-us-east-1a-01"
registration_key: "your-registration-key"

metadata:
  name: "marvin-us-east-1a-01"
  cluster: "production"
  region: "us-east-1"
  environment: "production"
  labels:
    team: platform
    datacenter: us-east-1a

control_plane:
  address: "control-plane.example.com:443"
  tls:
    enabled: true
    ca_cert_file: /etc/marvin/ca.crt

authentication:
  secret_key:
    key: "your-secret-key"

capabilities:
  enabled:
    - "network.*"
    - "kubernetes.*"
    - "prometheus.*"
    - "linux.*"
    - "os.*"

logging:
  level: info
```

### Run

```bash
./marvin -config /etc/marvin/config.yaml
```

---

## Configuration reference

| Section            | Key                  | Description                                                                |
| ------------------ | -------------------- | -------------------------------------------------------------------------- |
| `agent_id`         |                      | Unique ID for this Marvin instance.                                        |
| `registration_key` |                      | Organisation-scoped key for registration.                                  |
| `metadata`         |                      | Host metadata (name, cluster, region, environment, labels).                |
| `control_plane`    | `address`            | gRPC endpoint of the Control Plane.                                        |
|                    | `tls`                | TLS settings (enabled, CA cert, insecure skip verify).                     |
| `authentication`   | `secret_key`         | Simple shared-secret authentication.                                       |
|                    | `mtls`               | Mutual TLS authentication (cert + key files).                              |
| `capabilities`     | `enabled`            | List of capabilities or wildcards (e.g., `network.*`, `kubernetes.pod.*`). |
| `kubernetes`       | `kubeconfig_path`    | Path to kubeconfig (optional; uses in-cluster config if omitted).          |
|                    | `context`            | Kubeconfig context to use.                                                 |
|                    | `namespace`          | Default namespace for namespaced queries.                                  |
| `prometheus`       | `url`                | Prometheus base URL.                                                       |
|                    | `auth`               | Authentication: `none`, `basic`, `bearer`, or `header`.                    |
|                    | `guardrails`         | Max query range, max returned series, etc.                                 |
| `kafka`            | `bootstrap_servers`  | List of broker addresses.                                                  |
|                    | `tls`                | TLS / mTLS settings.                                                       |
|                    | `sasl`               | SASL mechanism (`PLAIN`, `SCRAM-SHA-256`, `SCRAM-SHA-512`, `GSSAPI`).      |
|                    | `guardrails`         | Max topics, max partitions, max response bytes.                            |
| `linux`            | `allowed_read_paths` | Whitelist of paths readable by `linux.file.read`.                          |
|                    | `max_read_bytes`     | Hard limit on file reads.                                                  |
| `graylog`          | `url`                | Graylog API URL (HTTPS only).                                              |
|                    | `auth`               | `none`, `token`, or `basic`.                                               |
|                    | `guardrails`         | Max query range, max results, max histogram buckets.                       |

---

## Development

### Project layout

```
.
├── cmd/marvin              # Main agent binary
├── cmd/provider-schema     # Utility to dump provider schemas
├── internal/
│   ├── config              # YAML config loading & validation
│   ├── controlplane        # gRPC client, auth, TLS
│   ├── metadata            # Host metadata autodetection
│   ├── provider/
│   │   ├── azure           # Azure Network Watcher / NSG
│   │   ├── kafka           # Kafka admin & metadata
│   │   ├── kubernetes      # K8s API resources
│   │   ├── linux           # System info, file reads, processes
│   │   ├── log             # Graylog search
│   │   ├── network         # DNS, ping, TCP, TLS, traceroute
│   │   └── prometheus      # PromQL queries
│   ├── registry            # Capability registry
│   └── task                # Task interface & result types
├── pkg/
│   ├── capability          # Capability naming & pattern matching
│   ├── log                 # Structured logging
│   └── proto/marvin        # Generated protobuf types
├── proto/
│   └── marvin.proto        # gRPC service definition
├── designs/                # Architecture & implementation plans
└── helm/                   # Kubernetes Helm chart
```

### Tests

All packages have unit tests. Where possible, integration tests spin up real dependencies via Docker / testcontainers.

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...
```

### Regenerate protobuf

```bash
make proto
```

---

## Security & guardrails

- **No CGO** eliminates a class of native dependency vulnerabilities.
- **Capability whitelist** — Only explicitly enabled capabilities are exposed.
- **Guardrails per provider** — Query limits, max response sizes, and timeout ceilings prevent accidental overload.
- **Authentication** — Supports secret-key and mutual TLS.
- **TLS** — All Control Plane communication is TLS 1.2+ with configurable CA verification.

---

## License

See `LICENSE`

---

## Contributing

- Open an issue to discuss large changes.
- All code must include tests.
- Follow Go idiomatic style (`gofmt`, `golint`).
- Keep the agent dependency-free: prefer pure Go over shelling out.
