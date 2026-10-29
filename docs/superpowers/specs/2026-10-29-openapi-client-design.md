# Generated qBittorrent client

Approved scope: migrate the provider's existing endpoints to an OpenAPI-generated
Go client. Keep Terraform resources, attributes, IDs, import behavior, state,
and the qBittorrent 5.3.0 / Web API 2.16.2 compatibility baseline unchanged.

## Approach

Maintain a reviewed OpenAPI 3.2.1 specification in `api/qbittorrent.yaml`, derived
from upstream tag `release-5.3.0`, commit
`9213075dd5f7b352208361c6a84c1c115efc898e`. Cover login, application/API versions,
allowlisted preferences and partial updates, and category CRUD only. This is
a provider-specific subset, not an official or exhaustive upstream schema.

Generate models and HTTP operations with pinned `oapi-codegen` into
`internal/client/openapi`. Keep generated code checked in and verify reproducible
generation in CI. Developer builds and provider releases use checked-in code;
generation is only needed when updating the specification or generator.

Retain `internal/client` as the provider-facing adapter. Generated code owns
request paths, methods, form encoding and API model fields. The adapter owns
URL validation, proxy prefixes, Origin/Referer, bearer headers, per-instance
cookie jars, serialized session login, one read-only retry after 403, refused
redirects, timeouts/cancellation, sanitized diagnostics and the 4 MiB response
limit. Decode bounded response bytes into generated allowlisted models; never
expose unknown preference keys or secret response bodies. Preserve optional
pointers so omitted, false and zero remain distinct.

Alternatives considered: a full API specification adds unrelated coverage and
maintenance; generating only models leaves request construction handwritten.
The user selected generated operations and models for existing coverage.

## Verification

Run existing client and provider tests unchanged where possible. Add focused
contract coverage for form escaping, empty category paths, all managed preference
keys, response bounds, proxy prefixes and schema generation. Run vet/race tests,
generation drift checks, unchanged Terraform docs, and disposable qBittorrent
5.3.0 acceptance tests. Never test mutations against the live Arr installation.

## Implementation choices

Use `oapi-codegen` v2.8.1, pinned in the generation command. Use generated plain
HTTP operations rather than response helpers, so bounded reads and sanitized
errors remain under adapter control. Keep Go field names compatible through
schema extensions and aliases. No new Terraform settings, version tag or release.
