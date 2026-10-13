# archivist

[![Test](https://github.com/jo-hoe/archivist/actions/workflows/test.yml/badge.svg)](https://github.com/jo-hoe/archivist/actions/workflows/test.yml)
[![Lint](https://github.com/jo-hoe/archivist/actions/workflows/lint.yml/badge.svg)](https://github.com/jo-hoe/archivist/actions/workflows/lint.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/jo-hoe/archivist)](https://go.dev/)

Sort markdown files into a folder tree and tag them in YAML frontmatter — using
**System 1 Decision Models** (von, laya) that return *typed answers with
calibrated probabilities* instead of free text. No LLM cost, latency, or
hallucination; just a model that answers "which folder?" and "does this tag
apply?" and tells you how confident it is.

- **Sorts** each file by walking your target folder tree top-down, asking the
  model a Choice question at each level until it's confident enough to stop.
- **Tags** files from a candidate list you provide, writing them into standard
  YAML frontmatter — merged into existing frontmatter, never clobbering your
  values by default.
- **Never loses files.** Every move writes the destination, verifies it, and
  only then removes the source. Name collisions are suffixed (`doc.md` →
  `doc-1.md`); an existing file is never overwritten.
- Runs on **Windows, macOS, and Linux**, as a CLI, an interactive TUI, or in
  Docker (CPU or GPU).

## How it works

1. **Index** the whole input set up front (one snapshot), so a file is never
   processed twice in a run.
2. For each file: parse frontmatter → send the body to the model as "state" →
   **descend** the folder tree via Choice questions → optionally **decide tags**
   → merge frontmatter → **move** the file atomically.

The two bundled models, [von](https://github.com/wfzyx/von) (default) and
[laya](https://github.com/NandhaKishorM/laya), speak the same `/v1/systemone`
protocol (see [systemonemodels.org](https://systemonemodels.org/)), so switching
between them is just one flag. Adding a new model is [three files and a registry
entry](./CONTRIBUTING.md#adding-a-new-system-1-model) — nothing else changes.

## Quick start

### Option A — Docker Compose (batteries included)

Weights download once into `./weights/` on first run and are reused on later runs.
archivist waits for the model to become healthy before processing any files.

**von (default):**
```bash
mkdir -p in out weights    # drop .md files into ./in; results land in ./out
docker compose up --build
```

**laya** (identical experience, different backend):
```bash
mkdir -p in out
COMPOSE_PROFILES=laya ARCHIVIST_MODEL=laya docker compose up --build
```

> **GPU:** both models auto-detect the best available accelerator. To force a
> specific device, set `VON_DEVICE=cuda` in `compose/von.env` or
> `LAYA_DEVICE=cuda` in `compose/laya.env` before running `docker compose up`.

> **Tip:** put your defaults in a `.env` file so you don't retype them:
> ```
> COMPOSE_PROFILES=laya
> ARCHIVIST_MODEL=laya
> ```

### Option B — local binary

Start any model server locally and point archivist at it. The flags are identical regardless of model.

```bash
# Start von:
docker run --rm -p 8000:8000 -v ./weights:/weights \
  -e HF_HOME=/weights ghcr.io/wfzyx/von:cpu

# Or start laya (requires Python 3.10+):
pip install "laya[serve]" && laya-serve
```

Build archivist and run against it:

```bash
go build -o archivist ./cmd/archivist

# von:
./archivist run --source ./in --target ./out \
  --model von --model-url http://localhost:8000

# laya (same flags, just change --model):
./archivist run --source ./in --target ./out \
  --model laya --model-url http://localhost:8000

# Interactive TUI (works with any model):
./archivist tui --source ./in --target ./out --model laya

# With tagging and folder descriptions:
./archivist run \
  --source ./in --target ./out \
  --model laya \
  --tags-file examples/tags.yaml \
  --descriptions-file examples/descriptions.yaml
```

## Configuration

Three layers, lowest precedence first: **config file < `ARCHIVIST_*` env vars <
CLI flags**.

- **Config file:** copy [`config.example.yaml`](./config.example.yaml) to
  `config.yaml` (auto-loaded from the working directory) or pass
  `--config path.yaml`. The example documents every option.
- **Env vars:** every flag has one — `--model-url` →
  `ARCHIVIST_MODEL_URL`. Handy in Docker; keep secrets like `ARCHIVIST_API_KEY`
  out of files.
- **Flags:** run `./archivist run --help` for the full list.

### Key options

| Flag | Default | What it does |
|------|---------|--------------|
| `--source` | — | Directory to walk, or a text file listing input paths (required) |
| `--target` | — | Root folder to sort into; its subfolders are the taxonomy (required) |
| `--recursive` | `false` | Walk subdirectories of `--source` |
| `--model` | `von` | Model backend — `von` or `laya` (or any registered model) |
| `--model-url` | `http://localhost:8000` | Model server base URL |
| `--model-device` | `auto` | Compute-device hint for the model (`auto`/`cuda`/`mps`/`cpu`) |
| `--tags-file` | — | YAML of candidate tags; **enables tagging** when set |
| `--tagging-strategy` | `noul` | `noul` (per-tag yes/no, 0..n tags) or `choice` (pick one) |
| `--tagging-threshold` | `0.7` | Confidence needed to apply a tag |
| `--taxonomy-strategy` | `scan` | `scan` existing folders, or `declared` from a YAML tree |
| `--descriptions-file` | — | Per-folder descriptions for the `scan` strategy |
| `--descent-strategy` | `threshold` | `threshold` or `stayhere` stop logic |
| `--descent-threshold` | `0.6` | Confidence needed to descend into a subfolder |
| `--override-meta` | `false` | Replace existing frontmatter values instead of keeping them |
| `--on-overflow` | `error` | `error` or `crop` when a document exceeds the token budget |
| `--on-options` | `error` | `error` or `adapt` when a choice exceeds the option cap |
| `--dry-run` | `false` | Compute the plan but move/write nothing |
| `--no-preview` | `false` | Skip the preview + confirmation (set in Docker) |

### Tagging

Tagging runs **only** when you pass `--tags-file`. See
[`examples/tags.yaml`](./examples/tags.yaml). With the default **noul** strategy,
each tag whose yes/no confidence meets the threshold is applied; a file that
earns no tags gets **no `tags` key** at all. The **choice** strategy picks
exactly one tag.

### Taxonomy

- **scan** (default): archivist reads the real folders under `--target`. Give
  each a description with `--descriptions-file`
  ([`examples/descriptions.yaml`](./examples/descriptions.yaml)) so the model
  chooses well.
- **declared**: define the whole tree in one YAML
  ([`examples/taxonomy.yaml`](./examples/taxonomy.yaml)) via
  `--taxonomy-strategy declared --taxonomy-file …`.

## Models

archivist talks to any model that implements the `/v1/systemone` protocol. Two are
bundled; they behave identically from archivist's perspective — same flags, same
config keys, same Docker experience.

| Model | Type | Max input tokens | Notes |
|-------|------|-----------------|-------|
| **von** (default) | pre-built image `ghcr.io/wfzyx/von:cpu` | 8,192 (~32K chars) | CUDA image not published; build locally for CUDA/ROCm. |
| **laya** | built from `Dockerfile.laya` | 1,024 (~4K chars) with multilingual checkpoint (default); 512 with English; up to 8,192 via `--max-state-tokens 8192` | Tight for long docs — use `--on-overflow crop`. Set `LAYA_DEVICE` in `compose/laya.env`. |

> **Token budget** — each request counts the document body + question instructions + folder/tag option descriptions together. System 1 models return typed answers (a probability or a choice key), not generated text, so there is no output token cost.

### Switching models

```bash
# CLI — same flags regardless of model:
./archivist run --source ./in --target ./out --model laya --model-url http://localhost:8000

# Docker Compose — just change the profile:
COMPOSE_PROFILES=laya ARCHIVIST_MODEL=laya docker compose up --build
```

### GPU acceleration

Both models auto-detect the best available device. To override:

- **von:** set `VON_DEVICE=cuda` (or `mps`, `openvino:gpu`) in `compose/von.env`  
  Note: the published `:cpu` image uses OpenVINO and supports Intel GPU out of the box. For NVIDIA CUDA you must build the von image locally from source (not distributed due to large wheel size).
- **laya:** set `LAYA_DEVICE=cuda` (or `mps`, `xpu`) in `compose/laya.env` — same image handles all devices; no rebuild needed.

### Multilingual documents

Both models handle documents in any language — cross-lingual matching works well
(an English tag description like "invoices and receipts" will match a German
`Rechnung`). For best results with non-English documents:

- **von:** language is a server-side checkpoint choice (`VON_MODEL=multilingual` in `compose/von.env`)
- **laya:** set `language: multilingual` in `config.yaml` (forwarded on each request)

### Adding a new model

Three files, no framework changes. See [CONTRIBUTING.md](./CONTRIBUTING.md#adding-a-new-system-1-model).

## Re-running

Across runs archivist always re-decides and moves files to their current best
location — it keeps no manifest. Running again after you add folders or change
thresholds reshuffles files accordingly; running with no changes is a no-op.

## Development

```bash
go build ./...
go test ./...
go vet ./...
```

CI runs the test and lint suites on Linux, macOS, and Windows for every push and PR to
`main`. Dependabot proposes weekly dependency updates; minor/patch bumps
auto-merge once CI passes.
