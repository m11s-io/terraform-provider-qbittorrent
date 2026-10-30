# Task 0: Shared foundation handoff

**Branch:** `feat/remaining-options`
**Base:** `f5de2e7`
**Scope:** shared preference metadata/lifecycle, generated OpenAPI contracts, local fragments and validator tooling.

## Interfaces for workers

- `client.Preferences` remains an alias of generated `openapi.Preferences`; existing `SetPreferences(ctx, Preferences)` callers remain supported. The compatibility setter converts through `PreferencesPatch`, which drops fields absent from the patch model.
- `client.PreferencesPatch` aliases generated `openapi.PreferencesPatch`; `SetPreferencesPatch(ctx, PreferencesPatch)` sends the typed write model.
- `preferenceField` is keyed metadata with `name`, `wireKey`, scalar `kind` (`string`, `bool`, `int`, `float`), `tab`, `description`, integer min/max/sentinels/multiple-of, float min/max, string/int enums, `sensitive`, `readOnly`, `excludeFromDataSource`, and `capability`.
- Define task fields as keyed `preferenceField` literals in the task-owned `preferences_<tab>.go`. Use `integerMin` / `integerMax`, `integerSentinels`, `integerMultipleOf`, `floatMin` / `floatMax`, `stringEnums` / `integerEnums`, and `capability` as needed. `sensitive` marks state presentation but does not suppress writes; use `excludeFromDataSource` to suppress general data-source exposure. Passwords and API keys do not get descriptors.
- `allPreferenceFields()` joins in fixed order: legacy, downloads, connection, bittorrent, rss, advanced session, WebUI settings, advanced libtorrent. `validatePreferenceFields()` rejects duplicate Terraform names or wire ownership; `preferences_fields_test.go` calls it against the full aggregate.
- Each scalar tab file starts with an empty `Read` and `Patch` schema object in `api/schemas/<tab>.yaml`. Put writable fields in both. Put computed-only fields in `Read` only. `Preferences` composes all `Read` objects plus `preferences-read-only.yaml`; `PreferencesPatch` composes all `Patch` objects plus `credentials-patch.yaml`. This keeps response-only keys out of generated patches without root composition edits.
- Capability-tagged configured inputs are verified by a preferences pre-read before a write. A missing wire key produces an attribute diagnostic and no POST. Missing capability-tagged data-source fields become typed null; missing established fields remain errors.
- Generic schema, patch and refresh code supports Float64. Read-after-write decoding uses exact JSON float64 values; integer fields remain int64.

## OpenAPI fragment contract

- Scalar tab fragments: `Read` and `Patch` in `downloads.yaml`, `connection.yaml`, `bittorrent.yaml`, `rss.yaml`, `advanced-session.yaml`, `webui-settings.yaml`, and `advanced-libtorrent.yaml`.
- Task 5 fills `credentials-patch.yaml`; task 7 fills `watch-folders.yaml#/Read` and `#/Patch`; task 8 fills `webui-maintenance.yaml#/Read` and `#/Patch`.
- Task 9 fills `rss-rules.yaml#/Rule`, `#/Rules`, `#/SetRuleForm`, `#/RemoveRuleForm`, and `paths/rss-rules.yaml#/rules`, `#/setRule`, `#/removeRule`. The root already references all these fragments. Generated operation IDs are `getRSSRules`, `setRSSRule`, and `removeRSSRule`; form fields are `ruleName` and JSON-string `ruleDef` for set, `ruleName` for remove.
- `scripts/bundle_openapi.go` resolves local YAML references into ignored `api/.qbittorrent-bundled.yaml` before oapi-codegen runs. The bundled output is worktree-local, so isolated workers do not overwrite each other. `api/oapi-codegen.yaml` retains all declared component models, including the distinct patch model.

## Security and upstream references

The upstream baseline remains qBittorrent `5.3.0`, Web API `2.16.2`, commit `9213075dd5f7b352208361c6a84c1c115efc898e`. The existing allowlist model continues to ignore unknown keys. The pinned `src/webui/api/appcontroller.cpp` GET/patch contract returns credentials among ordinary preferences; the fixture now includes canaries for `mail_notification_password`, `proxy_password`, `dyndns_password`, `web_ui_password`, and `web_ui_api_key`. Password patch fragment remains empty for Task 5 to fill. RSS operation skeletons follow the pinned `src/webui/api/rsscontroller.cpp` action contracts described in the design and plan.

## Verification

- TDD RED: the initial metadata test failed to compile because `allPreferenceFields`, keyed metadata and duplicate validation did not exist. It passed after implementing the shared registry.
- `go test -race ./...` — passed after final lifecycle changes.
- `go vet ./...` — passed after final lifecycle changes.
- Official OpenAPI schema-base validation — passed for both source and bundled specs using `jsonschema==4.25.1`, `PyYAML==6.0.3`, and `https://spec.openapis.org/oas/3.2/schema-base/2026-08-30`.
- `make generate` — passed. Two consecutive generations produced identical `client.gen.go` SHA-256: `33830203d8edfcdba0ad76af5a628822a8c0e67c2008d3633fc5885c5bbe478c`.
- `make generate-check` before commit reported the expected difference from the base generated file; rerun after committing the generated output.
- `make generate-check` after commit — passed with no generated drift.
- `make docs-check` — passed after retaining the existing resource-only Speed rate-limit description text.
- Disposable Terraform and OpenTofu acceptance — passed in the coordinator’s integration run.

## Review follow-up: read-only ownership

- Added `TestReadOnlyPreferenceIsDataSourceOnly`, which injects a read-only descriptor and asserts it is absent from the resource schema, present and computed in the data-source schema, and excluded from patch entries. The test failed before the fix because the descriptor appeared as a resource attribute.
- Resource schema generation now omits every `readOnly` descriptor. The data source retains those fields as computed outputs; apply/patch read-only guards remain as defense in depth.
- `go test ./...` — passed after the fix.
- `go test -race ./...` — passed after the fix.
