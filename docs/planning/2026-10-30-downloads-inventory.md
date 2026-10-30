# Downloads preferences inventory (qBittorrent 5.3.0)

## Scope and baseline

Inventory source is qBittorrent commit `9213075dd5f7b352208361c6a84c1c115efc898e` (tag `release-5.3.0`), not upstream `HEAD`. Provider baseline is `a73a3a1` (release 0.3.0). The provider currently exposes only `save_path`, `temp_path`, `temp_path_enabled`, and `auto_tmm_enabled` for Downloads (`internal/provider/preferences_resource.go`; summary at `docs/preferences-by-tab.md:56`). All other configuration entries below are missing. No provider code was changed for this inventory.

The upstream Options page loads through `loadPreferences()` and saves the entire options object through `app/setPreferences` (`src/webui/www/private/views/preferences.html:2491-2655,3007-3061,3523-3538`). The API GET and setter mappings are `src/webui/api/appcontroller.cpp:175-240,600-734`. Setter handling is per-key and does not require unrelated keys, which matches this provider's partial-patch behavior.

## Field inventory

All fields listed here are persistent application preferences unless marked otherwise. Types are JSON/API wire types. Enums are case-sensitive strings unless noted. The WebUI does not provide numeric units for these controls.

| Control | API key | Wire type and accepted values | Read/write behavior and planning notes |
| --- | --- | --- | --- |
| Torrent content layout | `torrent_content_layout` | string enum: `Original`, `Subfolder`, `NoSubfolder` | GET returns enum string. Setter converts with fallback `Original`; expose as validated string enum. |
| Add to top of queue | `add_to_top_of_queue` | boolean | Direct read/write. |
| Do not start automatically | `add_stopped_enabled` | boolean | Direct read/write; this is the inverse-sounding “start stopped” UI checkbox, but the wire value itself means add stopped. |
| Torrent stop condition | `torrent_stop_condition` | string enum: `None`, `MetadataReceived`, `FilesChecked` | GET enum string; setter fallback is `None`. |
| Merge trackers on duplicate | `merge_trackers` | boolean | Direct read/write. |
| Delete source `.torrent` after add | `auto_delete_mode` | integer enum: `0` never, `1` after successful add, `2` also when addition is cancelled | The UI combines two checkboxes into this one integer. Read interprets `>0` as delete and `===2` as also delete on cancel; write composes 0/1/2. Add a strict 0..2 validator (native setter casts without a range check). |
| Preallocate all files | `preallocate_all` | boolean | Direct read/write. |
| Append `.!qB` to incomplete files | `incomplete_files_ext` | boolean | Direct read/write. |
| Keep unselected files in `.unwanted` | `use_unwanted_folder` | boolean | Direct read/write. |
| Default torrent management mode | `auto_tmm_enabled` | boolean | Existing field. `true` means Automatic; native getter/setter invert the session's internal “disabled by default” flag. |
| On category change, relocate torrent | `torrent_changed_tmm_enabled` | boolean | Missing. `true` relocates; `false` switches torrent to Manual Mode. Getter/setter invert an internal disable flag. |
| On default save path change, relocate affected torrents | `save_path_changed_tmm_enabled` | boolean | Missing. `true` relocates; `false` switches affected torrents to Manual Mode. |
| On category save path change, relocate affected torrents | `category_changed_tmm_enabled` | boolean | Missing. `true` relocates; `false` switches affected torrents to Manual Mode. |
| Use category paths in Manual Mode | `use_category_paths_in_manual_mode` | boolean | Missing; controls relative save-path resolution against category path instead of default path. |
| Default save path | `save_path` | string path | Existing. Server path; no local filesystem validation or path normalization by API handler. |
| Enable incomplete path | `temp_path_enabled` | boolean | Existing. |
| Incomplete path | `temp_path` | string path | Existing. |
| Back up torrent files | `torrent_files_backup_enabled` | boolean | Missing; independent enable flag. |
| Torrent backup directory | `torrent_files_backup_dir` | string path | Missing; stored/read even when backup disabled. |
| Move finished torrent backup | `torrent_files_finished_backup_dir_enabled` | boolean | Missing; separate enable flag. |
| Finished torrent backup directory | `torrent_files_finished_backup_dir` | string path | Missing; stored/read even when finished-backup movement disabled. |
| Remove backup when removing torrent | `remove_torrent_file_backup` | boolean | Missing. |
| Automatically add torrents from watched folders | `scan_dirs` | JSON object `map[path]value` | Special legacy/deprecated format; details below. Setter replaces the watched-folder set, so this is a whole-object collection, not scalar partial ownership. |
| Enable excluded file names | `excluded_file_names_enabled` | boolean | Missing. |
| Excluded file names | `excluded_file_names` | string, newline-delimited patterns | GET joins native list with `\n`; setter splits on `\n`. Preserve the raw newline-delimited string (including empty lines) unless deliberately validating normalization. |
| Email notification on completion | `mail_notification_enabled` | boolean | Missing. |
| Sender address | `mail_notification_sender` | string | Missing; API setter stores raw string. |
| Recipient address(es) | `mail_notification_email` | string | Missing; UI permits semicolon-separated recipient groups and comma-separated addresses inside each group. Do not model as a Terraform list without conversion. |
| SMTP server | `mail_notification_smtp` | string | Missing; UI accepts DNS/IP and optional `:port`; port is embedded in string, not a separate wire field. |
| SMTP encryption | `mail_notification_encryption_type` | string enum: `None`, `STARTTLS`, `SMTPS` | Missing; native conversion fallback is `SMTPS`. Validate the UI enum. UI describes default ports 25, 587, 465 respectively; those are descriptive defaults, not separate settings or enforced units. |
| SMTP authentication enabled | `mail_notification_auth_enabled` | boolean | Missing. |
| SMTP username | `mail_notification_username` | string | Missing. |
| SMTP password | `mail_notification_password` | string credential | Native GET returns this value and setter accepts it as a string. Do not store it in ordinary Terraform state or expose it in data source output. If supported by the provider/framework, expose only as a write-only resource argument, send it in the patch, and never reconcile/read it into state. Framework baseline is v1.19.0; confirm its write-only support before implementation. Otherwise omit with explicit documentation rather than leaking the server-returned secret. |
| Run program on torrent added | `autorun_on_torrent_added_enabled` | boolean | Missing. |
| Program on torrent added | `autorun_on_torrent_added_program` | string command/template | Missing; native setter trims leading/trailing whitespace. Preserve server-normalized value on refresh. |
| Run program on torrent finished | `autorun_enabled` | boolean | Missing. |
| Program on torrent finished | `autorun_program` | string command/template | Missing; native setter trims leading/trailing whitespace. |

### `scan_dirs` object shape and semantics

The key is each monitored directory path. Values are deliberately mixed JSON types:

* integer `0`: monitored torrent's save location is the watched directory itself (UI choice `watch_folder`);
* integer `1`: use the default save location (UI choice `default_folder`); this is also emitted on GET when the stored override is empty;
* string: use this custom save path (UI choice `other`).

The UI trims both watched and custom paths, skips empty watched paths, and emits the object with all table rows. On GET, native code emits integer `0` when save path equals watched path, integer `1` when the save path is empty, otherwise the save-path string. On setter, integer 0 explicitly sets watched path and disables AutoTMM; integer 1 leaves the existing/default save parameters as they are; a string sets that custom path and disables AutoTMM. Invalid paths are silently caught/ignored by native code. Omitted old entries are removed after processing the new object. This legacy API shape is easy to clobber and may be omitted by newer APIs; treat it as a whole-map setting with explicit ownership and test an empty map, not as independent path-key Terraform attributes.

### Non-preference action

“Send test email” is a button action (`preferences.html:456,2190-2209`, POST `api/v2/app/sendTestEmail`), not persistent configuration. Do not model it as an attribute on the singleton preferences resource. A separate action/resource would be a distinct future feature.

## Validation, normalization, and operational effects

The frontend applies no numeric units to Downloads fields. On save it translates UI enums and checkbox combinations to the API values above. The native handler uses enum fallback conversion for layout/stop condition and SMTP encryption; invalid values therefore may be silently normalized to `Original`, `None`, or `SMTPS`. `auto_delete_mode` is an unchecked integer cast, so provider-side validation is important. Watched folder path errors are swallowed by native setter. External program strings are trimmed by native setter. Other scalar strings and booleans are forwarded to preference setters without API-layer range clamping.

The HTTP API treats keys independently: GET exposes effective values, POST mutates only keys supplied. The current provider's normal apply path serializes a typed allowlist, skips null attributes, then read-after-write compares every managed value and reports if the server normalized/rejected it (`internal/provider/preferences_resource.go:150-197,203-239`). Add fields to the generated OpenAPI `Preferences` allowlist and regenerate `internal/client/openapi/client.gen.go`; otherwise typed JSON unmarshal silently drops outbound keys and readback cannot reconcile them. Do not add secret fields to the GET/state allowlist.

These settings are persistent and do not require an application restart in this handler; the frontend reloads the page after save. Several settings have effects beyond the setting itself: changing default paths/AutoTMM policies can relocate torrents, backup flags affect future `.torrent` lifecycle, watch-folder patches replace the complete watch-folder map, and external-program/mail settings enable server-side actions on later events. No provider-side clamping should be invented beyond enum/range validation needed to avoid native fallback or unchecked casts. Read-after-write remains the server normalization check.

## Suggested implementation slices and tests

1. **Scalar add-torrent controls and schema contract.** Add layout, queue-top, stopped, stop-condition, merge trackers, auto-delete mode, preallocation, incomplete extension, and unwanted folder to `api/qbittorrent.yaml`, generated client model, and `preferenceFields`. Add choice/range validators. Tests: each enum allowed values and invalid value rejection; `auto_delete_mode` 0/1/2 roundtrip and invalid -1/3 rejection; explicit `false` and zero are serialized while null is omitted; client GET decode/set patch preserves unknown server fields policy.
2. **Saving management and backup paths.** Add the four missing TMM/category-path booleans and five backup fields, retaining existing paths and `auto_tmm_enabled`. Tests: false-valued keys still patch; all path strings roundtrip; enabled flags remain independent from stored paths; no local path existence requirement; read-after-write mismatch diagnostic for normalized path.
3. **Watched scan directory map.** Model `scan_dirs` explicitly as mixed `map[string]any` or a typed custom wire value; it cannot be a scalar generated preference pointer without a custom representation. Isolate conversion/validation and patch serialization. Tests: integer 0/1 remain JSON numbers (not booleans/strings), custom string path remains string, empty map replaces/removes all watches, omitted watch entry is removed, and GET 0/1/string shapes survive state/patch roundtrip. Test malformed server entries without panic and document native silent-ignore behavior.
4. **Excluded names.** Add enabled bool and newline-delimited string. Tests: multiple lines, one line, empty string, and intentional blank line retain wire representation; explicit false is sent.
5. **Email settings and credential boundary.** Add nonsecret SMTP fields and enum. Resolve password as a write-only argument in the resource schema, outside GET model/state reconciliation; send only when configured. Tests: SMTP enum validation; sender/recipient/server strings roundtrip unchanged; write-only password reaches patch but is absent from state/plan display/data source and never appears in diagnostics/logs. If framework cannot guarantee that contract, exclude password and test that returned server credentials are discarded.
6. **External program settings.** Add added/finished enabled+program pairs. Tests: enabled toggles and templates roundtrip; native whitespace trimming is reported as normalization; null relinquishes ownership.
7. **Docs and acceptance coverage.** Update `docs/preferences-by-tab.md`, resource docs, examples, and acceptance coverage. One integration test should patch/read every new scalar group without sending an all-preferences replacement; a separate isolated test should verify `scan_dirs` replacement semantics and removal. Keep test SMTP actions out of managed preference tests because it sends email.

## Source references

All upstream refs below are at commit `9213075dd5f7b352208361c6a84c1c115efc898e`.

* WebUI controls: `src/webui/www/private/views/preferences.html:169-359,361-472`.
* Scan-folder object conversion: `preferences.html:2078-2150`.
* Load conversion: `preferences.html:2491-2648`.
* Save conversion and final `app/setPreferences`: `preferences.html:3007-3061,3523-3538`.
* Native GET keys and values: `src/webui/api/appcontroller.cpp:175-240`.
* Native patch setters/fallbacks/replacement behavior: `appcontroller.cpp:600-734`.
* Enum definitions: `src/base/bittorrent/torrentcontentlayout.h:45+`, `src/base/bittorrent/torrent.h:120+`, `src/base/torrentfileguard.h:65+`, `src/base/net/smtpencryptiontype.h:39+`.
* Provider baseline field allowlist/apply/readback: `internal/provider/preferences_resource.go:28-70,150-239`; generated allowlist model: `internal/client/openapi/client.gen.go:33-125`; current per-tab list: `docs/preferences-by-tab.md:52-61`.
