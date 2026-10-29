# Verification

Verification record dated **2026-10-29** for the qBittorrent 5.3.0 compatibility
baseline. Toolchain versions and results below describe that verification run.

## Baseline and toolchain

- qBittorrent tag `release-5.3.0`, peeled commit
  `9213075dd5f7b352208361c6a84c1c115efc898e`.
- Container: `ghcr.io/linuxserver/qbittorrent@sha256:75ecbb1c89ab7b0a21c4bde40b4820e4f1649d2d56d83b91913422155e8d2a5f`.
  Both disposable runs verified `v5.3.0` and Web API `2.16.2`.
- Go `1.27.3`, verified as the latest stable on that date using `https://go.dev/dl/?mode=json`.
- Direct dependencies resolved from the module proxy as the latest stable on that date:
  Framework `v1.19.0`, plugin-go `v0.31.0`, plugin-testing `v1.16.0`.
  Runtime and test imports updated with `go get -u -t ./...`; exact transitive
  versions/checksums are in go.mod/go.sum.
- Terraform `1.15.9` and OpenTofu `1.13.2`, darwin/arm64 acceptance runs.
  CI uses the same pinned CLI versions on linux/amd64.
- tfplugindocs `v0.25.0`, actionlint `v1.7.12`, GoReleaser `v2.19.0`.

## Successful checks

| Command | Result |
| --- | --- |
| `go test ./...` | Unit suite passes; real acceptance is opt-in |
| `go test -race ./...` | Pass |
| `go vet ./...` | Pass |
| `gofmt` and `git diff --check` | Clean |
| `python3 tests/acceptance/run.py` | OpenTofu real acceptance + shipped examples init/validate/plan pass |
| `TF_ACC_TERRAFORM_PATH=<terraform-1.15.9> python3 tests/acceptance/run.py` | Terraform real acceptance + shipped examples init/validate/plan pass |
| `make docs` / `make docs-check` | Schema-generated documentation; reproducibility checked |
| `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` | Workflow lint passes |
| `goreleaser check` | Pass |
| `goreleaser release --snapshot --clean --skip=publish,sign` | Five platform ZIP artifacts and checksums; no publication/signing |

The real suite covers session and Bearer preferences lifecycles, read-only data
source, import, no-op plans, external drift, relinquishing ownership, preferences
destroy preserving settings, category lifecycle/import/rename/missing refresh,
invalid credentials, nested lifecycle ordering/missing ancestors, and category
deletion preserving a stopped tracker-free torrent and its `hello` file.

Unit tests additionally cover CSRF headers, URL/auth configuration and environment
precedence, isolated session handling, bounded reauthentication, no write replay,
malformed responses, errors without secret leakage, cancellation, timeout,
redirect refusal, duplicate categories, partial updates, false/zero/null behavior,
normalization errors, and rejected-write state preservation.

## CI

The [baseline validation run](https://github.com/m11s-io/terraform-provider-qbittorrent/actions/runs/40873858731)
passed unit/race tests, vet, documentation reproducibility, workflow lint,
cross-platform snapshots, and Terraform/OpenTofu acceptance on Linux/amd64.
Local acceptance also passed on macOS/arm64.

Snapshot checks skip signing and publication. Release signing and registry
registration are described in [development documentation](development.md).

## OpenAPI client migration

Verified on **2026-10-29** on the migration branch:

- OpenAPI **3.2.1** document validated against the official
  [3.2 schema-base dated 2026-08-30](https://spec.openapis.org/oas/3.2/schema-base/2026-08-30),
  including its referenced JSON Schema dialect, using `jsonschema==4.25.1` and
  `PyYAML==6.0.3`. `oapi-codegen` v2.8.1 consumes the document directly.
- `go test -race ./...`, `go vet ./...`, and `make build` passed after migrating
  all nine HTTP operations. New contract coverage exercises form escaping,
  empty category save paths, all allowlisted preferences, false/zero values,
  unknown secret fields, and the exact 4 MiB response boundary.
- Disposable acceptance and example init/validate/plan checks passed with
  **OpenTofu 1.13.2** and **Terraform 1.15.9**, against the same digest-pinned
  qBittorrent **5.3.0 / Web API 2.16.2** image above. Both session and Bearer
  lifecycles were exercised; live Arr services were not used.
- Schema-generated Terraform documentation has no changes. The handwritten
  development documentation adds the regeneration workflow.
- `make generate-check` passed; a deliberate staged artifact change was rejected,
  then restored and clean regeneration verified. `make docs-check`, workflow
  `actionlint` v1.7.12, formatting and `git diff --check` passed.
- Independent read-only review found no runtime blockers. Its one schema wording
  correction (configured category paths rather than effective paths) was applied.

This specification covers the provider subset; it is not a claim of full OpenAPI
3.2 generator feature support or complete qBittorrent API coverage.
