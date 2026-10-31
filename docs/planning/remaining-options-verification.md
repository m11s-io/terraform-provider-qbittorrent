# Remaining Options verification

Implementation follows `docs/superpowers/plans/2026-10-30-remaining-options.md`
and its design, starting at planning commit `f5de2e7` on provider baseline
`a73a3a1` / 0.3.0. Upstream evidence uses qBittorrent commit
`9213075dd5f7b352208361c6a84c1c115efc898e` exclusively.

## Coverage and ownership

The reviewed `2026-10-30-options-save-keys.json` matrix assigns every indexed
save key and additional inventoried control an owner, disposition, reason and
concrete evidence reference. `TestOptionsDispositionMatrixHasVerifiedOwnership`
checks those assignments against the actual registry and verifies evidence
paths. The index is not a claim that every control has runtime effect on every
linked-library/platform build.

Ordinary preferences retain partial ownership and native wire units. Specialized
resources cover the watched-folder map, write-only service passwords, explicit
WebUI maintenance, and individual RSS rules. Passwords/API keys are discarded
from public read models. Write-only values are read from Config, versioned, and
covered by ephemeral plan/state canary checks. Sensitive configured command
text/custom headers may remain in sensitive resource state and machine-readable
plans; these values are excluded from the general data source.

## Disposable verification boundary

The pinned image is
`ghcr.io/linuxserver/qbittorrent@sha256:75ecbb1c89ab7b0a21c4bde40b4820e4f1649d2d56d83b91913422155e8d2a5f`.
It reports qBittorrent 5.3.0 / API 2.16.2; Linux, bitness 64,
libtorrent 2.1.2.0, Boost 1.92.0, Qt 6.12.0, OpenSSL 4.1.0, zlib 1.3.2.
Only loopback WebUI ports are published. Discovery/forwarding are disabled;
ordinary hooks, SMTP/proxy/DynDNS, trackers and RSS downloading are not enabled.
Restart tests use only disposable containers and restore their original values.
Torrent hashes and RSS feed inventory are compared around collection/rule tests.

Listener and authentication transitions use a separate stateful local API
fixture under both Terraform 1.15.9 and OpenTofu 1.13.2. It closes the old listener,
loses the mutation response, delays target verification, checks retained state,
reconnects using the new provider URL/credentials, and counts writes across
refresh, follow-up apply and no-op plans. This proves provider/CLI recovery
behavior, not every native deployment's proxy, certificate or listener setup.
No infrastructure, release tags or live application configuration was changed.

## Runtime limitations

- See `../advanced-libtorrent.md` for exact compile predicates. Key presence
  proves API/readback support, not runtime application. The pinned build does
  not expose `TORRENT_USE_I2P` through buildInfo, so that flag is not claimed tested.
- Common enum subsets deliberately omit disk-I/O value 4 and write-through
  value 2 until their build capabilities can be established safely.
- Disk-cache values above 1536 MiB can normalize on 32-bit servers despite the
  universal 64-bit cap of 33554431 MiB; this build is 64-bit.
- The legacy watched-folder setter cannot reset an existing custom/watch-folder
  destination with integer 1. The resource rejects this transition before
  mutation; explicitly remove/clear the entry in one apply and re-add default
  mode in a second apply. Destroy itself preserves watches.
- RSS torrent tags use set semantics. Rules containing nonempty TLS certificate,
  private-key or DH material can be read/imported but updates are rejected before
  writing: the provider never stores or replays that material.
- WebUI maintenance is a staged targeted operation. Update provider connection
  inputs after it; no Kubernetes Service/port publication or TLS/auth bypass is
  performed automatically. Write-only passwords have no remote drift detection.

## Implementation decisions

1. Added finite float sentinels so ratios support exactly -1 or nonnegative
   values; otherwise valid disabled values would be rejected or negative
   fractions could drift.
2. Allowed bounded native integer/enum transport models to use int/int32 while
   retaining int64 for large/unbounded counters. A wrong width assertion could
   miss a transport bug; literal large-byte fixtures also cover that boundary.
3. Assigned omitted `web_ui_upnp` coverage to ordinary WebUI settings, since it
   is outside the endpoint resource's ownership list. A different ownership
   choice would require a migration; only explicit false is exercised.
4. Diagnosed unsupported legacy default-folder transitions before writing,
   rather than performing an implicit multiwrite reset. This costs an explicit
   staged user transition and avoids uncertain intermediate resets.

5. Conditional runtime behavior outside the pinned build remains documented
   stored/readback coverage; a different build may accept an inactive setting.
6. Deployment-specific listener/proxy/certificate/IP-ban transitions remain
   outside the fixture claim. Provider recovery is tested, but an environment
   mismatch may require manual recovery. No live transition was authorized.
7. The final reviewer confirmed the same explicit two-apply watched-folder
   limitation in decision 4; its cost remains staged user work.
8. The final reviewer accepted documenting the narrower 32-bit disk-cache cap;
   values above it can still normalize with a diagnostic on that platform.
9. Write-only password drift is intentionally undetectable; versioned writes
   avoid unsafe credential readback and replay, at the cost of not detecting
   external password changes.

## Final checks

The completed code is `0f5e791` over base `f5de2e7`; later commits contain
verification/tooling/documentation updates. All task reviews completed. The
whole-branch review identified two Important RSS issues (tag ordering and
preservation of unsupported TLS material); both were fixed with failing-then-
passing regressions and approved in one scoped follow-up review. No Critical,
Important or deferred Minor code findings remain.

| Check | Evidence |
| --- | --- |
| Full unit/race suite | `go test -race -timeout 120s ./...` passed after the RSS fix wave |
| Static analysis and build | `go vet ./...`, `make build` passed |
| Generation | `make generate-check` passed; field annotation audit had no model diff |
| Official OpenAPI 3.2 validation | Source and bundled documents passed schema-base `2026-08-30`, using Python 3.14, jsonschema 4.25.1, PyYAML 6.0.3 |
| Module/workflow consistency | `go mod tidy` left go.mod/go.sum unchanged; actionlint v1.7.12 passed |
| Terraform 1.15.9 | Complete disposable suite passed, including native tag deduplication, credential plan/state canaries, watched folders, rule CRUD, and independent maintenance fixture |
| OpenTofu 1.13.2 | Same complete disposable suite passed |
| Examples | Both CLIs initialized, validated and planned all shipped coverage/resource/maintenance examples independently; maintenance examples were not applied to the shared server |
| Release configuration/snapshot | GoReleaser v2.19.0 check and local snapshot passed; publish/sign disabled; snapshot code commit `0f5e791` |
| Captured output canaries | Final CLI/race logs had no matches for known credential fixture canaries or generated API-key patterns; acceptance also scans ephemeral plan/state values |
| Documentation | Regenerated from schema/source examples; clean `make docs-check` is run after committing this record |

The final maintenance CLI fixture additionally loses the auth response, delays
verification, reconnects using new provider credentials, and verifies no password
replay under a simulated threshold-one failed-login policy. The native server's
listener/auth transitions were not changed; this remains the documented fixture
boundary above.
