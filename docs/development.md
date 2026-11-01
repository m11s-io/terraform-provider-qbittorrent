# Development

## Compatibility

The provider supports qBittorrent 5.3.0 / Web API 2.16.2. The API contract is based
on upstream commit `9213075dd5f7b352208361c6a84c1c115efc898e` from `release-5.3.0`.
Application and API versions are checked before resources are configured.

Use the Go version specified in [go.mod](../go.mod). Docker and Terraform are
required for disposable acceptance tests. The CI workflow also tests OpenTofu.

## Build and test

```sh
make test
make race
make vet
make build
```

Tests use local HTTP fixtures unless explicitly run through the disposable
acceptance runner. Do not point acceptance tests at an existing server. See
[acceptance test instructions](../tests/acceptance/README.md).

## API implementation

The client uses form-encoded requests and typed allowlists. Preference writes are
partial JSON patches; omitted fields are untouched, while false and zero remain
explicit values. Reads are bounded, redirects are refused, and TLS, Origin and
Referer checks remain enabled. Each client owns its session cookies. Only reads
may reauthenticate automatically; mutations are never replayed automatically.

Public read models exclude passwords and API keys. Write-only password inputs
come from Config and are sent only when their version counters change. Avoid
logging request bodies, authentication headers, response bodies, or Terraform
state. Use synthetic credentials in tests.

New fields must match upstream wire names, units, enum values, native bounds and
normalization behavior. Add literal wire fixtures, sparse-write checks, and
read-after-write tests. Cover configured values, null ownership, drift and any
build-dependent behavior. See [settings coverage](coverage.md) and the
[resource guides](preferences-by-tab.md).

## Generate API code

[api/qbittorrent.yaml](../api/qbittorrent.yaml) is a maintained OpenAPI 3.2.1 subset,
not an official upstream specification. Edit schema/path fragments, then run:

```sh
make generate
make generate-check
```

The generator resolves local YAML references into an ignored bundle and uses the
pinned oapi-codegen version. Generated Go code is checked in; normal builds do
not run generation. Do not edit `internal/client/openapi/client.gen.go` manually.

Validate source and bundled documents against the official schema-base:

```sh
uv run --python 3.14 --with-requirements scripts/requirements-openapi.txt python scripts/validate_openapi.py
```

Python dependencies are tooling-only. Exact versions are pinned in
`scripts/requirements-openapi.txt`.

## Generate documentation

The Terraform Registry documentation is generated from schemas and source
examples using the version pinned in `GNUmakefile`:

```sh
make docs
make docs-check
```

Edit schemas, examples or templates instead of generated files under
`docs/resources`, `docs/data-sources` and `docs/index.md`. Handwritten guides belong
outside those generated paths. Format examples with `terraform fmt -recursive examples`.

## Releases

Run unit, race, acceptance, generation and documentation checks before releasing.
The release workflow uses GoReleaser to build platform archives, generate
checksums and sign them with the configured signing key. A version tag triggers
publication. Local snapshot verification does not publish or sign:

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=publish,sign
```

See [Terraform Registry publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing).
