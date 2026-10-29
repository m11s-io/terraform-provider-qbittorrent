# Implementation and verification evidence

Implementation date: 2026-10-29. Local checkout remains on `main`, as explicitly
requested. This is source/release readiness evidence, not Registry publication.

## Baseline and toolchain

- qBittorrent tag `release-5.3.0`, peeled commit
  `9213075dd5f7b352208361c6a84c1c115efc898e`.
- Container: `ghcr.io/linuxserver/qbittorrent@sha256:75ecbb1c89ab7b0a21c4bde40b4820e4f1649d2d56d83b91913422155e8d2a5f`.
  Both disposable runs verified `v5.3.0` and Web API `2.16.2`.
- Go `1.27.3`, verified latest stable using `https://go.dev/dl/?mode=json`.
- Latest stable direct dependencies resolved from the module proxy:
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

## Independent review and decisions

One independent read-only whole-change review found two important issues:
failed preferences writes returned planned state, and nested category creation
could silently create unmanaged ancestors. Regression tests reproduced both;
fixes passed the full unit/race and real acceptance suites. The review's category
path-description finding was treated as important because incorrect path
expectations affect torrent placement; schema/docs now describe exact resolution.
No review findings remain deferred. No second review was substituted for tests.

Implementation decisions:

- Work directly on local `main` per user instruction; no worktree created.
- URL is schema-optional but required after environment resolution, enabling
  QBITTORRENT_URL-only configuration. Missing/invalid URLs fail Configure.
- Only reads replay after bounded reauthentication. Expired-session writes need
  a rerun of apply, avoiding duplicate mutation side effects.
- Optional per-category incomplete paths are deferred from v1; global incomplete
  path preferences are available.
- Rate limits must be 0 or representable multiples of 1024, preventing upstream
  truncation/clamping and repeated drift.
- Nested category ancestors must already exist; descendant categories prevent
  deletion. These guards require explicit parent references/leaf-first ordering
  and are not transactional against concurrent external hierarchy mutations.

## Limits and remaining publication steps

Acceptance was executed locally on macOS/arm64. The CI matrix is configured for
Linux; a green local run does not assert that hosted CI has already run. Registry
credentials/public-key enrollment and the first authorized version tag remain
human/account steps, detailed in development.md. Snapshot signatures were skipped;
the release workflow follows the existing infrastructure-owned GPG convention.
No live qBittorrent deployment, actual download, tracker or Vault secret was used.
