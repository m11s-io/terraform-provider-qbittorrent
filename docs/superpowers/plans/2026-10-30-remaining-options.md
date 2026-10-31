# Remaining qBittorrent Options Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Implement the approved plan task-by-task; use the explicitly requested model `gpt-6-luna`, reasoning effort `high`, for implementation workers. Steps use checkbox (`- [x]`) tracking.

**Goal:** Cover all persistent configuration represented by the supplied Options screenshots without changing current production values.

**Architecture:** Modular tab schemas and field registries feed the existing flat preferences singleton. Specialized resources own watched-folder collections, write-only credentials, WebUI connection transitions and individual RSS rules. A single integrator owns composition, generation, registration and release integration.

**Tech Stack:** Go 1.27.3; Terraform Plugin Framework 1.19.0; OpenAPI 3.2.1; oapi-codegen 2.8.1/runtime 1.8.0; Terraform 1.15.9/OpenTofu 1.13.2 acceptance.

**Spec:** [remaining-options-design.md](../specs/2026-10-30-remaining-options-design.md)

## Global constraints

- Base `a73a3a1` / provider `0.3.0`; upstream `9213075dd5f7b352208361c6a84c1c115efc898e` / qBittorrent 5.3.0 / Web API 2.16.2.
- Preserve existing resource names, IDs, attributes, partial ownership, explicit false/zero and import/destroy behavior.
- Do not edit `infra/arr`, cluster charts, Vault, pfSense, running applications or release tags while implementing this plan.
- Source all wire keys, defaults, units, enum values, limits and sentinels from pinned upstream; record references in task docs.
- Never decode response secrets into public models/state; write-only inputs come from Config and use version counters.
- Generated files are never manually edited. All writes remain partial form-encoded JSON patches; no full preferences replacement.
- Action buttons are accounted for separately; they are not ordinary persistent attributes. The supplied screenshots are reference controls, not desired production values.

## Review focus

1. Bytes versus UI KiB/MiB, int64 versus float64, fractional ratios, explicit -1/0/empty values: task 0 and every numeric worker need literal wire fixtures.
2. Credentials in upstream GET, write-only values, custom headers/commands: tasks 0/5/8 need state/plan/log canary checks and no secret replay.
3. Coupled or whole-object setters, especially share-limit flags and `scan_dirs`: tasks 3/7 must test noncommutative changes, empty maps, unmanaged data and normalized state.
4. Endpoint/auth changes can succeed before the response/read fails: task 8 must mutate once, reconnect safely, retain useful state/diagnostics and never disable security.
5. Hidden/platform/libtorrent-dependent and read-only fields: tasks 0/4/6 must distinguish unsupported, stored-but-restart-pending and computed-only values without silently claiming enforcement.

## Source inventories

- [Downloads](../../planning/2026-10-30-downloads-inventory.md)
- [Connection, BitTorrent and RSS](../../planning/2026-10-30-network-rss-inventory.md)
- [WebUI and Advanced](../../planning/2026-10-30-webui-advanced-inventory.md)
- [Mechanical save-key coverage index](../../planning/2026-10-30-options-save-keys.json)

The index records 235 distinct literal save-handler keys: 35 already mapped in
0.3.0 and 200 missing/derived/special keys requiring classification. It excludes
clientdata keys, buttons, runtime outputs and expressions not written as literal
assignments. Do not use the raw count as a completion assertion. `bittorrent_protocol`
is assigned in the Speed save block but belongs to the Connection UI.
The design/plan take precedence over implementation suggestions in the raw
inventories (notably numeric share-limit sentinels versus competing enable inputs).

## Parallel ownership and schedule

One coordinator and three Luna/high workers can run concurrently. Foundation
must finish before workers fork. Create isolated worktrees from its commit, one
per worker; do not let workers share a changing checkout. Preserve unrelated
working-tree changes. No agent may spawn its own agents.

| Wave | Workers | Prerequisite |
| --- | --- | --- |
| Serial | 0: foundation/integrator | Approved design/plan |
| A | 1 Downloads; 2 Connection; 3 BitTorrent + RSS preferences | Foundation green |
| B | 4 Advanced session; 5 WebUI + credentials; 6 Advanced libtorrent | Merge/regenerate wave A, full unit/race suite green |
| C | 7 watched folders; 8 WebUI maintenance; 9 RSS rules | Shared resource/secret helpers green; task 8 consumes task 5 |
| Serial | 10 integration, acceptance and review | All worker handoffs |

Each worker owns its task-named schema fragment, provider definition/test files,
client files if specified, fixture, manual doc and example. Only the coordinator
edits `api/qbittorrent.yaml`, `internal/provider/preferences_fields.go`,
`internal/provider/provider.go`, existing generic lifecycle code, generated
`client.gen.go`, `go.mod`, root examples/CI runners and generated docs.

Workers may regenerate and adjust local aggregate wiring solely in their own
worktree to run tests. Their handoff commits must exclude those integration-only
edits; report exact registration changes needed. Coordinator imports the owned
files, wires each module once, regenerates, and runs tests before the next wave.
This avoids repeated manual resolution of generated files.

## Task 0: Shared foundation (serial)

**Owner:** coordinator or one Luna/high worker. **Files:** generic resource/data-source/validator files; new `internal/provider/preferences_fields.go`, `preferences_fields_test.go`; root OpenAPI composition and empty `api/schemas/*.yaml`; existing client/preferences.go and tests.

**Interfaces produced:**

- Preserve `type client.Preferences = openapi.Preferences` for nonsecret reads and existing `SetPreferences(context.Context, Preferences) error` callers.
- Add `type client.PreferencesPatch = openapi.PreferencesPatch` and `SetPreferencesPatch(context.Context, PreferencesPatch) error` for typed patches including explicitly supplied secrets. Patch model excludes computed/read-only fields.
- `preferenceField` uses keyed literals and metadata: `name`, `wireKey`, `kind`, `tab`, `description`; integer bounds/sentinel choices, float bounds, string/int enums, `sensitive`, `readOnly`, `excludeFromDataSource`, and optional capability identifier. Scalar kinds are `string`, `bool`, `int`, `float`. Keep all established names/wire values. Sensitive commands/headers may persist in their sensitive resource attributes when configured, but are excluded from general data-source outputs; actual credential fields are write-only and absent from state altogether.
- `allPreferenceFields() []preferenceField` deterministically joins `legacyPreferenceFields`, `downloadsPreferenceFields`, `connectionPreferenceFields`, `bittorrentPreferenceFields`, `rssPreferenceFields`, `advancedSessionPreferenceFields`, `webUISettingsPreferenceFields`, `advancedLibtorrentPreferenceFields`. No runtime `init()` registration or nondeterministic ordering.
- Initialize every scalar worker field slice as empty and its tab schema fragment as an empty object in the foundation commit, so isolated workers have a compiling shared base. Foundation owns initial creation only; after its commit, the task named below exclusively fills its fragment.
- Root schema composes existing fields plus local `api/schemas/{downloads,connection,bittorrent,rss,advanced-session,webui-settings,advanced-libtorrent}.yaml`. Separate local read-only and secret-patch fragments. Keep `x-ui-tab` and `x-go-name` throughout.

**Special schema/operation handoff (root composition stays integrator-owned):**

| Fragment seeded/referenced by task 0 | Filling owner | Root/generated contract |
| --- | --- | --- |
| `api/schemas/credentials-patch.yaml`, empty object | Task 5 | `PreferencesPatch` allOf references this file; response `Preferences` never does. Password property wire names are `mail_notification_password`, `proxy_password`, `dyndns_password`. |
| `api/schemas/watch-folders.yaml`, named empty objects `Read` and `Patch` | Task 7 | Root `Preferences` allOf refs `#/Read`; root `PreferencesPatch` allOf refs `#/Patch`. Worker fills only `scan_dirs` there, using integer 0/1 or string union. No generic resource field descriptor owns `scan_dirs`; specialized resource does. Its client decodes the generated model from the existing bounded app/preferences operation. |
| `api/schemas/webui-maintenance.yaml`, named empty objects `Read` and `Patch` | Task 8 | Root read refs `#/Read` for listener/username values; root patch refs `#/Patch` for same plus write-only `web_ui_password`. Secret is absent from Read. Specialized resources alone own writable fields. |
| `api/schemas/rss-rules.yaml`, named schema objects `Rule`, `Rules`, `SetRuleForm`, `RemoveRuleForm` | Task 9 | Root components `RSSRule`, `RSSRules`, `SetRSSRuleForm`, `RemoveRSSRuleForm` reference the corresponding local definitions; response schemas are rule-specific, not general Preferences. |
| `api/paths/rss-rules.yaml`, named PathItems `rules`, `setRule`, `removeRule` | Task 9 | Root paths `/rss/rules`, `/rss/setRule`, `/rss/removeRule` use local PathItem `$ref`s. Operations are `getRSSRules`, `setRSSRule`, `removeRSSRule`; form keys are `ruleName`/JSON-string `ruleDef` for set and `ruleName` for remove. Task 0 seeds typed operation skeletons from the pinned controller; task 9 completes optional read parameters/Rule schema. |

Foundation proves local model/PathItem references generate correctly. A worker
fills its owned fragment and runs `make generate` in isolation; because the root
already references that fragment, no root edit is needed for these standard
contracts. Generated diffs stay local and are omitted from the worker handoff.
The integrator imports owned fragment/adapter files, regenerates root output,
and registers the resource in the same integration commit. New unforeseen
endpoint requirements are returned as explicit wiring instructions, not silently
added to root by a worker.

- [x] Add tests first: existing HCL/import fixture remains valid; 1.75 ratio and zero round-trip; int64 `torrent_file_size_limit = 104857600` stays exact; secret GET canaries are absent; read-only keys never enter patches; duplicate wire ownership fails; new unsupported data-source fields can be null while existing missing required keys still diagnose.
- [x] Watch the new cases fail, then implement metadata/type/model separation and generic lifecycle extensions. `preferenceValue`, schema creation, apply and refresh must all support Float64.
- [x] Preserve 4 MiB bounded reads, cookie/Bearer auth, base paths, TLS checks, CSRF headers, read-only retry and no write replay. Add a filtered client fixture containing every upstream secret key.
- [x] Run `go test -race ./...`, `go vet ./...`, `make generate-check`, `make docs-check`, both disposable acceptance runners. Root generation may change code layout but cannot change existing behavior.
- [x] Add `scripts/validate_openapi.py` for official 3.2 schema-base validation, with pinned `jsonschema==4.25.1` and `PyYAML==6.0.3` tool dependencies and the dated schema reference used by current verification. Document a reproducible invocation; do not add these Python libraries to provider runtime dependencies.
- [x] Commit foundation; record SHA and worker interfaces before dispatching wave A.

## Task 1: Downloads scalars

**Own files:** `api/schemas/downloads.yaml`, `internal/provider/preferences_downloads.go`, `preferences_downloads_test.go`, `downloads_acceptance_test.go`, `docs/planning/downloads-implementation.md`, `examples/coverage/downloads.tf`.

**Consumes:** task 0 field metadata and patch/read composition. **Produces:** `downloadsPreferenceFields` containing missing public Downloads scalars from the inventory. Do not duplicate four existing mapped fields. Exclude `scan_dirs` (task 7) and password (task 5).

- [x] Write table-driven wire/state cases for layout/stop enums, auto-delete mode 0/1/2, backup flags/paths, TMM policies, excluded-name text, SMTP nonsecret fields and external-program hooks. Assert only managed keys are sent.
- [x] Verify failing tests; add exact schema fields and native enum/bound validation. Preserve command trim semantics and mark command templates sensitive; no hook execution or email delivery in tests.
- [x] Test false, zero, empty path/text, multiline patterns, normalization diagnostics, unrelated settings unchanged and null relinquishment.
- [x] Run targeted and full unit tests; disposable acceptance keeps hooks/mail disabled while verifying stored templates/settings.
- [x] Commit owned files and hand off source evidence, test results and wiring instructions.

## Task 2: Connection controls

**Own files:** `api/schemas/connection.yaml`, `internal/provider/preferences_connection.go`, `preferences_connection_test.go`, `connection_acceptance_test.go`, `docs/planning/connection-implementation.md`, `examples/coverage/connection.tf`.

**Produces:** `connectionPreferenceFields`. Cover protocol enum, four connection/upload limit fields, I2P enable/host/port/mixed/PeX, proxy nonsecret fields and routing flags, filtering and `banned_IPs`. Existing listen_port/upnp remain in legacy module. Password belongs to task 5.

- [x] Test protocol values and `-1`/positive limits with explicit rejection of zero (native normalizes it); use wire-key `banned_IPs` exactly, despite mixed case.
- [x] Test proxy enum `None`/`SOCKS4`/`SOCKS5`/`HTTP`, auth-enabled false, configured disabled dependents, proxy ports and IP-filter paths. Preserve raw newline text and native normalization errors.
- [x] Implement fragments/metadata after RED. Native port bounds must come from the relevant setter rather than reusing listen-port validation everywhere.
- [x] Acceptance leaves proxy/I2P/filter/UPnP features disabled or uses local fixture files; never route test internet traffic or enable router forwarding. Assert secret response filtering.
- [x] Document restart requirements for I2P PeX and the Random-port action boundary; commit owned files.

## Task 3: BitTorrent and RSS preferences

**Own files:** `api/schemas/{bittorrent,rss}.yaml`, `internal/provider/preferences_{bittorrent,rss}.go`, corresponding unit/acceptance files, `docs/planning/bittorrent-rss-implementation.md`, `examples/coverage/{bittorrent,rss}.tf`.

**Produces:** `bittorrentPreferenceFields`, `rssPreferenceFields`. Includes privacy, checking concurrency, queue/slow limits, ratio/duration/mode/action, tracker text/URL, RSS reader/timing/article/autodownload/smart filters. Existing dht/pex/lsd stay compatible.

- [x] Write failing cases: encryption enum 0/1/2, queue maxima -1/0/positive as actually supported, slow thresholds KiB/s, timer seconds, ratio 1.75 and -1, seeding minutes -1/0, MatchAny/MatchAll and actions 0..3.
- [x] Implement numeric canonical fields for share limits; three enabled flags and fetched tracker text are read-only data-source outputs. Never send derived flags or `add_trackers_url_list` in a patch. Source-check sentinel normalization before choosing validation.
- [x] Test RSS units separately (refresh minutes, delay seconds), zero/minimum behavior established by native setters, blank/multiline filter strings and explicitly false enable flags.
- [x] Acceptance uses no tracker/feed network and no active torrents; keep RSS fetching and auto-download disabled. Seeding removal actions are tested on stopped disposable fixture data only, never user content.
- [x] Document every removal action's consequence and separate RSS rule resource ownership; commit owned files.

## Task 4: Advanced qBittorrent session controls

**Own files:** `api/schemas/advanced-session.yaml`, `internal/provider/preferences_advanced_session.go`, its unit/acceptance files, manual doc and `examples/coverage/advanced-session.tf`.

**Produces:** `advancedSessionPreferenceFields` for the Advanced qBittorrent section in the inventory. Do not take libtorrent controls from task 6 or WebUI listener controls from task 8.

- [x] Test storage/removal enums, interface/address writes and resolved-name read-only output, save intervals, file-size bytes, recheck/instance/display/peer/tag controls, embedded tracker/Python/start-paused/shutdown settings.
- [x] Literal unit fixture: UI 100 MiB maps to `torrent_file_size_limit = 104857600`, not 100. `shutdown_timeout = -1` is valid; values below -1 are rejected.
- [x] Implement supported public fields with restart/capability metadata. Interface validation uses server knowledge or reports native normalization, never runner NICs. Discovery endpoints can be typed read-only helpers if needed; no imperative connection changes.
- [x] Acceptance avoids moving content, switching resume backend, enabling trackers or changing bind interfaces. A dedicated disposable restart test validates persistence of explicitly restart-required options; no production restart.
- [x] Commit owned files with exact enum/native-setter evidence.

## Task 5: WebUI non-listener settings and password writes

**Own files:** `api/schemas/webui-settings.yaml`, `api/schemas/credentials-patch.yaml`, `internal/provider/preferences_webui_settings.go`, `credentials_resource.go`, its tests, `internal/client/credentials.go`, manual docs and examples.

**Produces:** `webUISettingsPreferenceFields`; `credentialsResource` implementing resource.Resource, registered by integrator as `qbittorrent_credentials`. General preferences own auth bypass/whitelists/ban/session limits, alternative UI, security/domain/header/reverse-proxy settings and nonsecret DynDNS settings. Task 8 owns listener, username/password.

- [x] Write secret canary tests before implementation: passwords never appear in GET models, data sources, state/plan JSON or diagnostics; only the requested changed version emits the secret; unrelated changes and destroy emit no secret writes.
- [x] Resource fields: `smtp_password_wo`, `smtp_password_wo_version`, `proxy_password_wo`, `proxy_password_wo_version`, `dyndns_password_wo`, `dyndns_password_wo_version`; singleton/import ID `application`. Require matching value/version when changing a secret. Read write-only values from Config; retain only version counters in state. Clearing an upstream-supported password uses explicit empty input plus version change.
- [x] Add public WebUI settings with exact native delimiters/ranges. Custom headers are sensitive; keep secret-containing text out of public data sources. Never expose computed plaintext API keys.
- [x] Verify initial create, versioned rotation, unchanged-version no-replay, null relinquishment, import and destroy-state-only behavior under Terraform and OpenTofu. Password fixtures are random and masked; no external SMTP/proxy/DynDNS requests.
- [x] Unit/fixture tests cover security flags without disabling real test-server protections. Commit owned files; supply root registration and secret-patch composition instructions.

## Task 6: Advanced libtorrent controls

**Own files:** `api/schemas/advanced-libtorrent.yaml`, `internal/provider/preferences_advanced_libtorrent.go`, unit/acceptance files, manual doc and `examples/coverage/advanced-libtorrent.tf`.

**Produces:** `advancedLibtorrentPreferenceFields` covering the entire libtorrent section, including decode/threads/cache/I/O, buffers/ports/resolver/security, choking/announcing, turnover/request/DHT/STUN/I2P controls. Inventory includes version/platform-conditional rows.

- [x] Write literal tests for wire bytes (`disk_queue_size=104857600` for UI 102400 KiB; socket buffers bytes) versus KiB watermarks, percent values, seconds, 0 defaults and permitted negative sentinels.
- [x] Add enum validators from pinned native enum definitions: disk I/O/read/write, mixed mode, slots behavior and choking. Record exact linked-library feature assumptions; do not blindly accept every option shown in another build.
- [x] Add every control to the fragment/metadata and fixtures. Enforce cross-field outgoing port min/max constraints only when known; preserve unknown values until apply validation.
- [x] Acceptance changes safe representative values only. Conditional features use supported/unsupported fixtures plus pinned build info; restart-required settings get stored-value and controlled restart coverage. No performance/network benchmark is required to prove persistence.
- [x] Commit owned files and list any capability not exercised by the pinned image explicitly.

## Task 7: Watched-folder collection

**Own files:** `api/schemas/watch-folders.yaml`, `internal/client/watch_folders.go`, `internal/provider/watch_folders_resource.go`, focused tests/docs/example.

**Produces:** typed `watchFoldersResource` registered by integrator as `qbittorrent_watch_folders`; client conversion helpers around the existing app endpoints. Resource owns only `scan_dirs`; exclude this key from normal writable preferences.

- [x] RED tests for map union 0/1/custom string, omitted versus empty collection, custom path equal to watched path normalization, invalid server values, and full-map replacement removing omitted entries.
- [x] Terraform shape: `folders = { "/watch" = { save_mode = "default" } }`; modes `default`, `watch_folder`, `custom`, with `save_path` required only for custom. ID/import `application`; document authoritative whole-map ownership. Destroy preserves server watches; explicit empty `folders = {}` clears them.
- [x] Implement exact legacy `scan_dirs` union conversion. Source says map replacement and invalid paths can be silently ignored: validate server-normalized round trips and fail useful diagnostics, rather than treating HTTP 200 as success.
- [x] Acceptance uses empty disposable directories and verifies no torrents/files are imported or removed. Test drift, import, empty map, deletion/relinquishment semantics.
- [x] Commit owned files and root registration instructions.

## Task 8: Explicit WebUI endpoint and authentication maintenance

**Own files:** `api/schemas/webui-maintenance.yaml`, `internal/client/webui_maintenance.go`, `internal/provider/webui_{endpoint,auth}_resource.go`, dedicated tests/manual maintenance example.

**Consumes:** task 5 secret/version helpers and existing `client.Config` / `client.New`. **Produces:** two resource constructors registered by integrator and an isolated candidate-client helper in its owned client file; listener/auth fields excluded from ordinary preferences ownership.

- [x] Write a real local fixture test that changes the listener port and deliberately loses the initial connection, then reconnects at `post_update_url` exactly once without replaying the mutation. A failure case reports applied/uncertain state and manual reconnect guidance without secrets.
- [x] Endpoint owns bind/port/HTTPS/cert-path/key-path and required target URL when endpoint changes. Auth owns `username`, `password_wo`, `password_wo_version`; enforce native username/password rules. Reauthenticate with candidate target credentials; never mutate the client used concurrently by other resources.
- [x] Test session and Bearer operation modes, unsupported self-credential transitions, candidate TLS validation failure, invalid bind/port, unchanged credentials not replayed, password absent from state, no-op plan and follow-up apply using the new provider URL/credentials.
- [x] Use staged targeted maintenance in examples. Do not weaken TLS, CSRF, hostname checks or auth to reconnect; do not edit a Kubernetes Service or publish a port automatically. Restart behavior stays explicit and confined to disposable fixtures.
- [x] Commit owned files and root registration instructions. A failure to verify safe reconnect blocks this task, not unrelated scalar coverage.

## Task 9: RSS auto-download rules

**Own files:** `api/schemas/rss-rules.yaml`, `api/paths/rss-rules.yaml`, `internal/client/rss_rules.go`, `internal/provider/rss_rule_resource.go`, focused tests/docs/example.

**Produces:** `rssRuleResource`, registered as `qbittorrent_rss_rule`. Resource ID is rule name; rename replaces unless an explicit native rename workflow is separately justified.

- [x] Read pinned `rsscontroller.cpp::{rulesAction,setRuleAction,removeRuleAction}` and `rss_autodownloadrule.cpp::{toJsonObject,fromJsonObject}` first. Required form keys: `setRule` uses `ruleName` plus JSON-string `ruleDef`; removal uses `ruleName`; reads use rules collection with the pinned optional filters.
- [x] Write tests for enabled/priority, regex/contain/not-contain/episode fields, `affectedFeeds` array, ignore days/smart filter, category/path/add-stopped/content-layout and native `torrentParams`. `lastMatch` and `previouslyMatchedEpisodes` are runtime history: expose computed where appropriate and preserve them on edits; never reset them because Terraform changed a pattern.
- [x] Implement typed, allowlisted rule definition and per-rule ownership. Test missing rule refresh, duplicate/adopt behavior, rule replacement semantics, null add-stopped/content-layout, drift, import and safe removal without deleting feeds or content.
- [x] Acceptance keeps RSS fetching/auto-downloading disabled and uses no external feed; rule CRUD alone must not add torrents. No bulk import/export or auto-discovery of unrelated rules.
- [x] Commit owned files and root schema/path/registration instructions.

## Task 10: Integration, final acceptance and release handoff (serial)

**Owner:** coordinator. **Files:** root composition/registration, generated client/docs, aggregate tests, acceptance runner, coverage matrix and verification record.

- [x] Merge handoffs by file ownership; reject duplicate keys/resource ownership. Regenerate after each wave and run the complete unit/race suite before advancing.
- [x] Replace the mechanical source index with a reviewed disposition matrix: every screenshot control is `existing`, `new preference`, `special resource`, `computed-only`, `action/non-config`, or `capability-limited` with reason/test reference. No blank/unaccounted rows. Existing Behavior/Speed coverage must remain green.
- [x] Run `make generate-check`, official OpenAPI 3.2 schema validation, `go test -race ./...`, `go vet ./...`, `make build`, both CLI disposable suites (new examples included), `make docs-check`, workflow lint and snapshot release checks. Verify clean generation/module/docs diffs.
- [x] Run state/plan/log canary scans for all supplied/generated write-only credentials. Sensitive configured templates/headers may exist in sensitive resource state and machine-readable plan JSON (document this), but never in ordinary display/diagnostics or general data-source outputs. Preserve native security checks and verify credential/endpoint transitions are never bundled into routine settings acceptance.
- [x] Fresh independent reviewer checks the final commit range and the five Review Focus items. Fix concrete critical/important findings with failing-then-passing tests; document unsupported runtime build cases honestly.
- [x] Commit completed implementation and verification. Publishing a provider tag and changing `infra/arr` are a later explicit step; no broad live apply is part of this plan.

## Worker dispatch template

Use one task per prompt with `fork_turns="none"`, `model="gpt-6-luna"`,
`reasoning_effort="high"`:

```text
Implement Task <N> only from docs/superpowers/plans/2026-10-30-remaining-options.md.
Read its design and relevant inventory first. Worktree: <absolute isolated path>.
Base: <foundation/integration SHA>. You own only the task's named files.
Do not touch aggregate/generated/release/live infrastructure files in your commit.
Regenerate locally as needed to test, exclude integration-only diffs from handoff.
Preserve the global constraints. Write failing tests first, implement, run targeted
and full tests. Do not spawn agents. Return owned-file commit SHA, tests/results,
source references, root wiring instructions, and any unresolved capability issue.
```

## Planning verification

Three Luna/high agents inventoried independent tab groups. Coordinator spot-checks
corrected UI/wire byte units and `current_interface_address` writeability; inventories
remain implementation evidence rather than permission for live changes. The plan
was checked for shared-file conflicts, endpoint/action distinction, credentials,
typed values, dependent setters and already-supported keys. No implementation or
runtime test is claimed by this planning deliverable.

## Implementation handoff

Implemented on `feat/remaining-options` in the isolated integration worktree.
See [verification and limitations](../../planning/remaining-options-verification.md)
and the reviewed save-key disposition matrix. All task and final reviews are
resolved; final native/CLI checks are recorded there. No provider tag, live
application configuration or infrastructure was changed.
