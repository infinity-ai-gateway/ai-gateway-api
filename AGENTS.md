# AGENTS.md — AI Gateway API

This file guides AI coding agents working on the `ai-gateway-api/` codebase (the control-plane API core of AI Gateway).

## Project overview

AI Gateway API is the **control-plane core component** of AI Gateway. It exposes Open APIs and Inner APIs for creating, storing, and distributing policies and configurations, and exports configuration consumed by the BFE data plane and Conf Agent.

AI Gateway system context (for orientation only):
- **AI Gateway API** (this repo): control plane, exposes APIs.
- **BFE** (`bfenetworks/bfe`): data plane, forwards traffic.
- **Conf Agent** (`bfenetworks/conf-agent`): fetches latest config and triggers BFE hot reload.
- **Dashboard** (`rainway-ai-gateway/ai-gateway-web`): Web UI for visual management.
- **Service Controller** (`bfenetworks/service-controller`): discovers and syncs Kubernetes backend services.

## High-level architecture

Entry point: `main.go`
- Parses flags (`-c conf_dir`, `-sc server_conf`, `-l log_dir`).
- Loads config via `stateful.LoadConfig` and initializes DB/Redis/dependencies.
- Registers routes via `endpoints.RegisterRouters` and starts the HTTP server with graceful shutdown.

Layered request flow:
1. **API layer** (`endpoints/`): gorilla/mux routers + negroni middleware.
   - `openapi_v1/`: user-facing Open APIs.
   - `innerapi_v1/`: internal APIs (config export, GSLB data, server data, etc.).
   - `middleware/`: recovery, logging, CORS.
2. **Model layer** (`model/`): business managers (e.g., API keys, AI routes, clusters, quotas, rate limits).
3. **Storage layer** (`storage/rdb/`): database access objects for MySQL/SQLite.
4. **Infrastructure** (`stateful/`): config, logging, metrics, DB/Redis clients, lifecycle.

## Directory structure and module relationships

| Directory | Responsibility |
|-----------|----------------|
| `endpoints/` | HTTP routing and handlers. `openapi_v1/` for Open APIs, `innerapi_v1/` for Inner APIs, `middleware/` for shared HTTP middleware. |
| `model/` | Business logic managers. Key sub-packages: `api_key`, `iai_route`, `iauth`, `ibasic`, `icluster_conf`, `imods`, `iprotocol`, `iroute_conf`, `iversion_control`, `quota`, `rate_limit_policy`, `route_rules`, `shared`. |
| `storage/rdb/` | Database access layer (DAO). Mirrors the domains in `model/`. |
| `stateful/` | Config loading (`config.go`), DB config (`config_database.go`), dependency init (`config_depends.go`), Redis config, logging, metrics, mock Redis, i18n, SQL recording. |
| `lib/` | Shared utilities: `xreq` (HTTP request/response), `xdb`, `xerror`, `validate`, `container`, `conf_load`, `http`, `logger`, etc. |
| `data/`, `conf/`, `static/`, `docs/` | Runtime data, TOML configs, dashboard static assets, documentation. |
| `design-docs/` | API definitions (`api-define/`), system design (`sys-design/`), and per-change records (`modifications/`). |
| `test/` | Test helpers and integration-related code. |
| `version/` | Version constant. |

## Build/test conventions

- **Go version**: 1.24 (`go.mod`).
- **Module**: `github.com/rainway-ai-gateway/ai-gateway-api`.
- **Build**: `make` downloads deps and builds the `ai-gateway-api` binary.
- **Test**:
  - `make test` runs all non-vendor packages.
  - `make test-model` runs `./model/...`.
  - `make test-model-cover-gate` enforces a 70% statement-coverage threshold on `model/`.
- **License headers**: `make license-check` / `make license-fix` use `license-eye`.
- **Docker**: `make docker` builds a local image; `make docker-push REGISTRY=...` builds and pushes multi-arch images.
- **DB initialization**:
  - MySQL: `mysql -u{user} -p{password} < db_ddl.sql`
  - SQLite: `db_ddl_sqlite.sql`
- **Start locally**: `./ai-gateway-api -c ./conf -sc ai_gateway_api.toml -l ./log`
  - API port: 8183 (default)
  - Monitor port: 8284 (default)

## Common modification patterns

### Add or modify an Open API
1. Add handler/controller under `endpoints/openapi_v1/<domain>/`.
2. Register routes in `endpoints/openapi_v1/endpoints.go`.
3. Update the corresponding `model/<domain>/` manager and `storage/rdb/<domain>/` DAO if persistence changes.
4. Update `design-docs/api-define/` and `design-docs/sys-design/` for non-trivial changes.
5. Add tests.

### Add or modify an Inner API
1. Add handler under `endpoints/innerapi_v1/<domain>/`.
2. Register routes in `endpoints/innerapi_v1/endpoints.go`.
3. Export/version-control logic usually touches `model/imods/` and `model/iversion_control/`.

### Add a new domain/model
1. Define manager under `model/<domain>/`.
2. Define storager interface in `model/<domain>/` (often using `itxn.TxnStorager`).
3. Implement DAO under `storage/rdb/<domain>/`.
4. Update `db_ddl.sql` and `db_ddl_sqlite.sql` with new tables/columns.
5. Add unit tests; coverage target is ≥ 70% for model code.

### Config export / version control
- Export logic: `model/imods/` and `model/iversion_control/`.
- Inner export endpoints: `endpoints/innerapi_v1/export_util/`.
- GSLB/server data endpoints: `endpoints/innerapi_v1/gslb_data/`, `endpoints/innerapi_v1/server_data/`.

### AI-specific features (token auth, AI route, rate limiting)
- Control-plane managers: `model/imods/`, `model/iai_route/`, `model/quota/`.
- Corresponding data-plane modules: `bfe/bfe_modules/mod_ai_token_auth`, `mod_ai_route`, `mod_ai_rate_limit`.
- When changing export formats or AI behavior, coordinate with `bfe/`.

### Design-first workflow (recommended for non-trivial changes)
Follow the six-step methodology in `design-docs/README.md`:
1. Create `design-docs/modifications/YYYYMMDD-<summary>/` with `change-summary.md` (and `api-changes.md` / `design-changes.md` if needed).
2. Update `design-docs/api-define/`.
3. Update `design-docs/sys-design/` and `design-docs/sys-design/summary.md`.
4. Implement code: endpoints → model → storage.
5. Add/update tests.
6. Summarize and decide whether to add a long-lived `design-docs/sys-design/details/` document.

## Agent guidelines

- **Follow `design-docs/README.md`** for non-trivial features; keep design docs and code in sync.
- **Prefer dependency injection** in managers; avoid hard-coding dependencies on `stateful.DefaultConfig` or `stateful.DefaultClientSet` so tests can mock them.
- **Use `itxn.TxnStorager`** for transactions; do not open ad-hoc DB transactions in managers.
- **Mock pattern**: use hand-written callback mocks (e.g., `fakeEntityStorager` with `createFn`/`fetchFn` fields) in `_test.go` files. See `TESTING.md` and existing `mocks_test.go` files.
- **Model coverage**: keep `model/` statement coverage ≥ 70%; current overall coverage is ~87%.
- **Do not commit generated files** such as `coverage.out` or the `ai-gateway-api.exe` binary.
- **Run tests**:
  - `make test-model` after model changes.
  - `make test-model-cover-gate` before submitting.
  - `make test` for full-package verification.
- **License headers**: all new source files need the Apache 2.0 / Rainway AI Gateway header. Use `make license-fix` if unsure.
- **Coordinate with `bfe/`** when changes affect AI modules, export formats, or config schemas consumed by the data plane.

## Useful references

- `README.md` / `README_CN.md` — project overview, quick start, architecture.
- `CONTRIBUTING.md` — workflow and code style.
- `TESTING.md` — unit-test organization, mock patterns, coverage targets.
- `design-docs/README.md` — six-step change methodology.
- `design-docs/sys-design/summary.md` — system design index.
- `Makefile` — build, test, Docker, and license targets.
