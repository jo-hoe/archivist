# Contributing to archivist

## Project structure

```
archivist/
├── cmd/archivist/main.go          # thin entry point
├── Dockerfile                     # archivist binary (distroless, static)
├── Dockerfile.laya                # laya model server (python:3.11-slim)
├── docker-compose.yml             # all model variants via COMPOSE_PROFILES
├── compose/<model>.env            # per-model server environment
├── examples/                      # sample tags / descriptions / taxonomy YAMLs
└── internal/
    ├── cli/                       # cobra+viper CLI and Bubble Tea TUI
    ├── config/                    # Config struct, Build(), Validate()
    ├── model/                     # System1 interface, transports, registry
    ├── frontmatter/               # YAML frontmatter parse/merge/render
    ├── taxonomy/                  # Provider strategies (scan, declared)
    ├── descent/                   # Choice-walk + StopStrategy
    ├── tagging/                   # Tagging strategies (noul, choice)
    ├── fileops/                   # Indexer + atomic Mover
    └── pipeline/                  # Per-file orchestration
```

Dependency direction is strictly acyclic:
`cli` → `config` → `pipeline` → {`model`, `taxonomy`, `descent`, `tagging`, `frontmatter`, `fileops`}

---

## Adding a new System 1 model

All models share the `/v1/systemone` wire protocol. Adding one is three steps — no
changes to the CLI, config validation, or pipeline are needed.

### 1. Create `internal/model/<name>.go`

Copy `von.go` or `laya.go` as a template. Set the defaults for your model and
choose the right state encoder:

```go
package model

// <name> implements the /v1/systemone protocol.
// See <upstream URL>.

const (
    <name>DefaultMaxStateTokens   = 8192
    <name>DefaultMaxChoiceOptions = 255
)

func new<Name>(cfg Config) (System1, error) {
    trans := newTransport(transportOptions{
        baseURL:    cfg.BaseURL,
        apiKey:     cfg.APIKey,
        model:      cfg.ModelID,
        encoder:    stringStateEncoder{},  // or bodyStateEncoder{} if your model wraps state in {"body":...}
        httpClient: cfg.HTTPClient,
    })
    return &client{
        name:  "<name>",
        trans: trans,
        caps: Capabilities{
            MaxStateTokens:   pick(cfg.MaxStateTokens, <name>DefaultMaxStateTokens),
            MaxChoiceOptions: pick(cfg.MaxChoiceOptions, <name>DefaultMaxChoiceOptions),
        },
    }, nil
}
```

Key decisions:
- **`stringStateEncoder`** — sends state as a bare JSON string (like von).
- **`bodyStateEncoder`** — wraps state as `{"body": ...}` (like laya).
- **`requestExtras`** — use this to forward model-specific request fields (e.g. `lang`, `max_len`, `device`). See `laya.go` for an example.
- **Answer shape** — `decodeResponse` in `transport.go` handles both the von top-level shape and the laya nested-`answers` shape automatically.

### 2. Register in `internal/model/registry.go`

```go
var registry = map[string]factory{
    "von":    newVon,
    "laya":   newLaya,
    "<name>": new<Name>,   // ← add this line
}
```

That's it for the Go side. The CLI exposes `--model <name>` automatically.

### 3. Add Docker Compose support

Create `compose/<name>.env` (model server environment — port, device, cache dir):

```env
# Environment for the <name> model service.
<NAME>_HOST=0.0.0.0
<NAME>_PORT=8000
<NAME>_DEVICE=auto
HF_HOME=/weights/hub
```

Add two profile services to `docker-compose.yml` — one for CPU, one for GPU — following the exact same pattern as `laya`. Both must:
- Register `model-server` as a network alias so archivist always finds them at `http://model-server:8000`.
- Depend on `init-weights` completing successfully.
- Include a healthcheck against `http://localhost:8000/health`.

GPU acceleration is handled via environment variables in `compose/<name>.env` (e.g. `<NAME>_DEVICE=cuda`), not by a separate image. If your model requires a different image for CUDA, add a separate profile service for that case — see `compose/von.env` for the pattern.

Activate with:
```bash
COMPOSE_PROFILES=<name> ARCHIVIST_MODEL=<name> docker compose up --build
```

---

## Running tests

```bash
go test ./...
go vet ./...
golangci-lint run ./...
```

CI runs the test suite on Linux, macOS, and Windows on every push and PR to `main`.

## Code style

- Low cyclomatic complexity — extract named helpers rather than deep nesting.
- No duplication — shared logic lives in `transport.go`, `client.go`, `limits.go`.
- Strongly typed — no `interface{}` in the public API surface.
- Idiomatic Go — `errors.Is`, `fmt.Errorf("...: %w", err)`, table-driven tests.
