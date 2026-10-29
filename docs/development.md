# Development contract

The compatibility baseline is qBittorrent **5.3.0**, tag `release-5.3.0`,
peeled commit `9213075dd5f7b352208361c6a84c1c115efc898e` (annotated tag
`05fcaa8f505f0657d5cd7b7c377282b63855e8ae`). Web API version: **2.16.2**.
The provider rejects other application/API versions rather than silently guessing.

Upstream API sources (all at the commit above):

- `src/webui/api/appcontroller.cpp`: `versionAction`, `webapiVersionAction`,
  `preferencesAction`, `setPreferencesAction`.
- `src/webui/api/authcontroller.cpp`: login/logout.
- `src/webui/api/torrentscontroller.cpp`: category CRUD.
- `src/webui/webapplication.cpp`: routing, sessions, authentication, CSRF, errors.
- `src/base/bittorrent/sessionimpl.cpp`: speed-limit units and category removal.

## HTTP and authentication

All paths below are relative to the configured WebUI base URL; `/api/v2/` is
appended, preserving any reverse-proxy base path. Reads use GET; writes use POST
with `application/x-www-form-urlencoded` bodies.

| Endpoint | Body / response |
| --- | --- |
| `auth/login` | `username`, `password`; success is HTTP 200 or 204, not the legacy `Ok.` body |
| `app/version` | Plain `v5.3.0` |
| `app/webapiVersion` | Plain `2.16.2` |
| `app/preferences` | JSON object; includes secrets which must never reach state/logs |
| `app/setPreferences` | `json` form parameter containing a partial JSON object |
| `torrents/categories` | JSON object keyed by category name, each has `name`, `savePath` |
| `torrents/createCategory` | `category`, `savePath`; duplicate/invalid name: 409 |
| `torrents/editCategory` | `category`, `savePath`; missing category: 404 |
| `torrents/removeCategories` | `categories` newline-separated names; missing is a successful no-op |

Category endpoints also support `downloadPathEnabled`/`downloadPath`. These are
currently omitted: global incomplete-download path settings remain
available through preferences. No category rename endpoint is used; names are IDs. Creating a nested category
implicitly creates missing ancestors upstream. The provider requires ancestors
to exist, avoiding unmanaged creations; reference parent resource names to order
lifecycles. An empty category path resolves as parent effective path plus category
basename. Nonempty relative paths resolve beneath the global default save path.

Invalid login: 401. An unauthenticated private endpoint: 403. CSRF or Host
validation failure: 401. Banned client: 403 with a descriptive plaintext body.
Other errors: 400 bad parameters, 404 missing, 409 conflict, 415 bad data, 405
wrong verb. Bodies are localized plaintext (or the HTTP reason phrase) and are
not exposed by the client because responses may contain secrets. Successful
mutations generally return empty 200/204 responses.

Session cookies use a `QBT_SID_` name prefix with an instance-specific suffix,
HttpOnly, path `/`, optional Secure and SameSite policy. Each provider owns an
in-memory cookie jar. Login is serialized; expired-session 403 triggers at most
one re-login for reads. Writes are never automatically replayed: rerun apply
following a session expiry. Redirects are refused, including same-origin ones.
Requests send Origin (scheme+host) and Referer (base URL) and keep TLS verification.
Host/CSRF protection remains enabled. Host validation also requires the Host
header port to match the server listener port: map equal ports in containers
or configure the reverse proxy to preserve the expected Host/Origin relationship.

API keys are supported via `Authorization: Bearer <key>`. An empty/unconfigured or
incorrect key does not authenticate. Bearer requests cannot access `auth/*`;
otherwise the target tag grants access to its registered controllers, including
preferences/categories. Keys must be bootstrapped outside this provider. Do not
use a custom `X-API-Key` header. The preferences response contains
`web_ui_api_key`; WebUI password/hash is not returned. Only explicitly allowlisted
nonsecret preferences are decoded by this client.

## Preferences

`setPreferencesAction` checks each key with `hasKey`: absent keys are unchanged.
Unknown keys may be silently ignored. Typed client fields and exact version checks
bound this contract. Resource ID/import ID is `application`. Only non-null fields
are managed; removing one relinquishes ownership. Import reads but manages no
fields until configured. Destroy only removes state.

| Key | Type / units |
| --- | --- |
| `save_path`, `temp_path` | string paths |
| `temp_path_enabled`, `auto_tmm_enabled` | bool |
| `dl_limit`, `up_limit`, `alt_dl_limit`, `alt_up_limit` | bytes/s, 0 unlimited; server quantizes to multiples of 1024; provider requires 0 or positive multiples of 1024, max 2147482624 |
| `dht`, `pex`, `lsd`, `upnp` | bool |
| `listen_port` | TCP/UDP port, 1..65535; random-port preference can override it |

Paths use server filesystem semantics. Use normalized absolute server paths.
Read-after-write detects normalization/rejection and reports the affected key
without values. Rate-limit validation avoids silent clamping/perpetual drift.

## Category deletion

Upstream `removeCategory` removes the named category **and all descendants**.
Torrents in those categories are reassigned to the parent category (or empty for
a top-level category); automatic torrent management can relocate their content.
No torrent or file deletion API is called. The provider refuses deletion when a
descendant category exists, so unmanaged descendants are not silently deleted.
This check is not transactional: avoid concurrent category hierarchy mutations.
Removing a category may still move files according to the server's TMM settings.

## Dependencies and checks

The Go version and dependencies are pinned in go.mod. The documentation generator
is pinned in GNUmakefile. See [verification results](verification.md) for tested
versions and commands.

Acceptance tests use a disposable digest-pinned 5.3.0 server and temporary
credentials. See [acceptance test instructions](../tests/acceptance/README.md).

## Generated OpenAPI client

`api/qbittorrent.yaml` is an OpenAPI **3.2.1** specification reviewed against the
exact upstream commit listed above. It covers only the provider's nine existing
operations and nonsecret preference/category models. It is a maintained subset,
not an official upstream specification or an automated C++ extraction.

`internal/client/openapi/client.gen.go` is checked in and generated by
`oapi-codegen` **v2.8.1**; its runtime is pinned to **v1.8.0** in go.mod. Normal
builds do not run the generator. Regenerate after modifying the spec or generator:

```sh
make generate
make generate-check
go test -race ./...
```

`generate.go` pins the generator version, and `api/oapi-codegen.yaml` configures
models and client generation. Generation uses the 3.2.1 document directly without
conversion; the covered constructs are shared with earlier OpenAPI versions.
Remote specification references are disabled. CI regenerates and rejects drift.

Generated operations own endpoint paths, HTTP methods and form serialization;
generated models own the wire keys and optional pointers. `internal/client`
retains its public API as an adapter, including cookie sessions, Bearer headers,
CSRF headers, proxy prefixes, bounded read-only reauthentication, redirect
refusal, timeout/cancellation and sanitized diagnostics. It reads bounded bytes
and decodes generated models directly instead of using the generated response
helpers, which buffer raw response bodies. Unknown preference keys are discarded
and must never be captured through generated additional-properties maps.

For a new supported field/endpoint, review the corresponding upstream C++ at the
baseline tag, update the specification and regenerate, then add the Terraform
mapping and contract/acceptance coverage. This migration adds no Terraform fields
or state schema changes.

## Publishing releases

1. Run the unit, race, acceptance, documentation and snapshot checks.
2. Configure the release workflow's `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` secrets.
   Register the corresponding public signing key with the provider registry.
3. Push a semantic-version tag to run the release workflow. It publishes platform
   ZIPs, the protocol-6 manifest, and signed SHA256SUMS covering both.
4. Register the provider repository with the registry and verify release webhook
   delivery and version ingestion.

References: [HashiCorp publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing),
[GoReleaser checksum extra files](https://goreleaser.com/customization/package/checksum/).
