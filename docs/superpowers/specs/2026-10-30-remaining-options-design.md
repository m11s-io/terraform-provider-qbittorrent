# Remaining qBittorrent Options coverage

## Intent and baseline

Plan the missing controls in the nine supplied Downloads, Connection, BitTorrent,
RSS, WebUI and Advanced screenshots for implementation by **gpt-6-luna / high**
workers. Planning only: do not release a version or alter live Arr preferences.

- Provider baseline: `a73a3a1`, released as `v0.3.0`.
- Upstream: tag `release-5.3.0`, commit
  `9213075dd5f7b352208361c6a84c1c115efc898e`; use `git show`, not checkout HEAD.
- Existing Go 1.27.3, framework 1.19.0, OpenAPI 3.2.1,
  oapi-codegen 2.8.1/runtime 1.8.0 pins remain authoritative.
- Existing Behavior/Speed controls and resource/state names remain compatible.

The three source inventories in `docs/planning/2026-10-30-*-inventory.md` are
part of this design. `2026-10-30-options-save-keys.json` indexes explicit Options
save-handler assignments, with baseline coverage and pinned source lines. This
mechanical index also contains platform/linked-library-dependent controls; it
is not a claim that every key has runtime effect on every build.

## Approach

Keep public, independently writable scalar preferences flat in
`qbittorrent_preferences`; continue `qbittorrent_webui_preferences` for Behavior
clientdata. Organize schema fragments, field definitions, tests and docs by tab.
Separate domains with different ownership or mutation semantics:

| Surface | Proposed Terraform interface | Contract |
| --- | --- | --- |
| Ordinary read/write fields | Existing `qbittorrent_preferences` | Partial ownership; omitted keys unchanged; false/zero/empty/-1 explicit |
| Fetched tracker list, resolved interface name | Computed data-source fields only | Never sent in patches |
| Watched folders | `qbittorrent_watch_folders`, singleton `application` | One owner of the complete `scan_dirs` map; no duplicate attribute in ordinary preferences |
| SMTP/proxy/DynDNS passwords | `qbittorrent_credentials`, singleton `application` | Write-only fields plus per-password version counters; never read secrets into state |
| WebUI username/password | `qbittorrent_webui_auth`, singleton `application` | Explicit maintenance update; bounded reauthentication with target credentials |
| WebUI bind/port/HTTPS paths | `qbittorrent_webui_endpoint`, singleton `application` | Explicit maintenance update with target URL and verified reconnect; no automatic TLS/auth bypass |
| RSS rule editor | `qbittorrent_rss_rule`, one resource per rule name | Own rule definition, not the whole rule collection; preserve match-history fields |
| Buttons: random port, test email, rotate/delete/copy API key, DynDNS Register | Documented action/non-configuration coverage | Not implicit side effects of normal apply; see action boundary below |

These are proposed new interfaces for the later implementation. A mapping plan
does not mean copying the screenshot values into production. In particular,
connection limits, deletion/seeding actions, proxy settings, hooks and security
flags stay unmanaged until users configure them.

### Why this shape

A giant concurrently edited preferences file would create merge conflicts and
encourage missed validators. One resource per UI tab would create several
owners of one flat API without solving secrets or whole-map replacement.
Modular definitions plus the existing singleton preserve compatibility while
specialized resources give collections, credentials and endpoint changes their
own lifecycle. A generic untyped JSON escape hatch is excluded: it defeats the
allowlist, documentation, validation and secret boundary.

## Shared foundation (must precede parallel implementation)

1. Move existing scalar definitions into an unchanged legacy module; create
   disjoint new tab modules and aggregate them deterministically. Reject duplicate
   wire keys/resource ownership at test time. Replace ad-hoc numeric `kind` names
   with metadata for type, bounds, enum, wire key, tab, capability and read mode.
2. Support `Float64` end-to-end for share ratios, with finite values only.
   Preserve integers as integers; no float coercion of 64-bit byte counters.
3. Split generated models into **nonsecret reads** and **write patches**.
   Response models/data sources must discard SMTP/proxy/DynDNS passwords and API
   keys, even though upstream returns them. Secret write fields must not be
   added to the public read allowlist. Keep adapter transport bounds/errors.
4. Compose OpenAPI models using local tab schema references and `allOf`. Retain
   every existing Go field name with `x-go-name`. Seed empty fragments/modules
   for workers. Only the integrator owns aggregate files and generated output.
5. Resource refresh stays strict for configured unsupported fields; diagnose
   before writing when support can be determined. Optional build-dependent
   **new** data-source fields may be null when absent. Missing established fields
   remain errors. Distinguish accepted/stored values from runtime features that
   require restart; never restart qBittorrent implicitly.
6. Add reusable finite-number, enum, integer/sentinel and cross-field validation.
   Use native setters to establish limits. Where evidence establishes only a
   C++ integer width, validate that width rather than inventing a tighter range.

## Wire semantics that workers must preserve

- Keep units in wire units for existing/same-name Terraform fields. E.g.
  `torrent_file_size_limit` is **bytes** (UI MiB); `disk_queue_size` and
  `socket_*_buffer_size` are **bytes** (UI KiB). Send-buffer watermarks are KiB;
  slow-torrent thresholds are KiB/s. Do not apply a global KiB conversion.
- `max_ratio` needs fractional values; `-1` disables. Seeding-time values are
  minutes; `-1` disables, zero is valid. Enabled booleans are derived upstream.
  Keep numeric sentinel fields as canonical writable inputs; expose the three
  `*_enabled` views as computed data-source outputs instead of competing inputs.
- `scan_dirs` wire values are integer 0 (watched directory), integer 1 (default)
  or custom path string. Terraform uses a typed `folders` map whose values have
  `save_mode = "watch_folder" | "default" | "custom"` and optional `save_path`.
  Empty map clears all; null omission does not. Exactly one resource owns it.
- Preserve raw newline-delimited text where that is the upstream API contract.
  Do not split/join with commas, reorder patterns, or erase intentional empty
  strings. Document native normalization and fail on rejected configured values.
- `current_interface_address` **is writable**. `current_interface_name` is a
  display output. Interface choices come from the server, not the runner host.
- Disabled connection/upload limits use `-1`; zero is normalized by native
  setters to `-1`, so reject zero rather than creating perpetual drift.
- Expose every conditional Advanced control in the manifest, but gate exact
  support against pinned native code/build info and controlled fixtures. Do not
  infer API support simply because a browser hides/shows a row.

## Secrets and connection changes

Use framework `WriteOnly` attributes read from **Config**, not Plan/State, and
nonsecret monotonically changed version counters. Proposed credentials inputs:
`smtp_password_wo`/`smtp_password_wo_version`,
`proxy_password_wo`/`proxy_password_wo_version`, and
`dyndns_password_wo`/`dyndns_password_wo_version`. Send only on create or the
corresponding version change; do not resend on unrelated updates. Null relinquishes
ownership without clearing the secret. Explicit empty string can clear a
password only if the upstream setter permits it. Never claim secret readback or
drift detection. Verify state/plan JSON contains no supplied secret.

WebUI auth gets `username`, `password_wo`, `password_wo_version`; password minimum
6 and username minimum 3/no colon match upstream. Endpoint resource owns
`web_ui_address`, `web_ui_port`, `use_https`, `web_ui_https_cert_path`,
`web_ui_https_key_path`, plus a provider-only `post_update_url`. HTTPS fields are
paths, not certificate/private-key contents. Mutate once, then reconnect/read
with a bounded candidate client; never replay uncertain writes. If verification
fails, say mutation outcome is uncertain and how to reconnect, without secrets.
Keep existing provider clients immutable for concurrent resources. Use a separate
targeted maintenance apply and update provider endpoint/credentials afterwards;
never include these transitions in ordinary combined fixture updates.

SMTP/proxy/DynDNS usernames remain normal preferences; command templates/custom
headers can contain secrets and should have sensitive metadata without appearing
in general data-source outputs unless intentionally documented. Passwords/API
keys never appear in generic debug output, generated examples, or diagnostics.
Authentication, HTTPS, CSRF, hostname checks and network isolation are never
disabled to make a test/reconnect pass.

## Actions and scope boundaries

- The Random button generates a port choice in the UI; the declarative input is
  `listen_port`. Do not add a rerandomizing resource or duplicate legacy
  `random_port` toggle. Preserve the current 1..65535 `listen_port` contract.
- Test email sends mail; API-key rotate/delete mutate credentials and return a
  secret; DynDNS Register opens an external registration flow; copy is a browser
  action. Record them as **not persistent configuration**, with endpoint/source
  references. No account registration/email/rotation during a normal apply.
- If explicit actions are requested later, design their invocation and secret
  destination separately. Framework actions exist, but CLI parity and returning
  a generated key securely require a separate decision. Do not hide rotation in
  a preference resource or publish a plaintext computed API key in general state.
- RSS rules are persistent configuration behind a separate modal, so include
  rule CRUD in the plan. Feed/folder CRUD, matching-article previews, refreshing
  feeds, importing/exporting all rules and fetching live feeds are separate
  surfaces; do not silently add them to ordinary preferences.

## Parallel execution boundary

Use high-reasoning Luna workers with isolated branches/worktrees and narrow
file ownership. Four available slots mean one coordinator plus **three workers
at a time**. Foundation is serial. Workers may regenerate in their own worktree
to compile/test, but exclude generated/aggregate files from handoff commits;
the coordinator regenerates once after merging each batch. No worker edits
root provider registration, central field aggregation, root schema, go.mod,
CI, generated docs, release/version files or live infrastructure.

The implementation plan defines exact task ownership and batches. Finish with
one integrator's full suite, generation/schema/docs checks, disposable acceptance
under both CLIs, and a fresh independent review before a separately authorized
release/adoption step.
