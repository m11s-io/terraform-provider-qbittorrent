# Options inventory: Connection, BitTorrent, and RSS

Baseline: provider `a73a3a1047a2e915779ea39a5b8a9c0a87449069` (released 0.3.0). Upstream reference is qBittorrent tag `release-5.3.0`, commit `9213075dd5f7b352208361c6a84c1c115efc898e`; use this commit rather than the vendor checkout's current HEAD. This inventory covers the requested missing Options controls in the Connection, BitTorrent, and RSS tabs. Existing controls to leave alone are Connection `listen_port` and `upnp`, BitTorrent `dht`, `pex`, `lsd`, and the complete Speed tab.

## API and provider contract

All actual settings are fields in `GET /api/v2/app/preferences` and are patched with `POST /api/v2/app/setPreferences` using form field `json` containing only configured keys. Upstream reads the fields into `AppController::getPreferences` and conditionally applies each key in `AppController::setPreferences`; partial updates are supported. Provider local contract is `api/qbittorrent.yaml` `Preferences`, generated client in `internal/client/openapi/client.gen.go`, and the shared `preferenceFields`/schema conversion in `internal/provider/preferences_resource.go` and `preferences_data_source.go`. The provider's allowlist intentionally omits sensitive WebUI credentials and should continue to omit `proxy_password` from both state and data source.

`preferenceFields` currently uses `string`, `bool`, `int64`-like integer kinds. Two important mismatches for this scope: disabled numeric limits are represented by upstream values `-1` (provider's generic number validator currently rejects negatives), and `max_ratio` is a real/decimal (`it.value().toReal()` upstream) while the provider's current numeric plumbing is integer-only. Add explicit kinds/schema conversions and validators before declaring those attributes. Preserve partial ownership: only configured attributes are sent, and false/zero/empty values must remain distinguishable from null/unset.

## Connection

Source UI and read/write references are `src/webui/www/private/views/preferences.html` Connection markup lines 495–690, UI populate lines 2645–2722, UI save lines 3063–3139, and `src/webui/api/appcontroller.cpp` read lines 240–294 and patch lines 730–833 (all at the pinned commit). Port-random button handler is `generateRandomPort` in preferences.html around line 2393. It uses browser crypto to choose a port >=1024 and only fills the `listen_port` input; it makes no request or independent server mutation. The provider should expose the port value, not an imperative random-button action.

| UI control | Wire key(s) | Type / values / units | Read/write and notes |
|---|---|---|---|
| Peer connection protocol | `bittorrent_protocol` | integer enum `0` TCP+µTP, `1` TCP, `2` µTP | Read/write. Though rendered on Connection tab near Speed controls, save code groups it under Speed. Scope it here per request; annotate as Connection in provider metadata/docs if preserving actual tab origin is preferred. No special restart annotation in this UI. |
| Global connection limit | `max_connec` | integer; positive enabled value, `-1` disabled sentinel | UI treats any `<=0` read as disabled and posts `-1` when checkbox off. Disabled numeric entry displays fallback `500`. Checkbox is UI state only; resource should model numeric preference directly with a validator allowing `-1` or positive integer. |
| Connections per torrent | `max_connec_per_torrent` | integer; positive or `-1` disabled sentinel | Same pattern, display fallback `100`. |
| Global upload slots | `max_uploads` | integer; positive or `-1` disabled sentinel | Same pattern, display fallback `8`. |
| Upload slots per torrent | `max_uploads_per_torrent` | integer; positive or `-1` disabled sentinel | Same pattern, display fallback `4`. |
| I2P enable, host, port | `i2p_enabled`, `i2p_address`, `i2p_port` | bool, string, integer port `0..65535` in HTML (`type=number`, min 0, max 65535) | Read/write; host and port are disabled in UI unless I2P enabled, but all values are still saved. Model independently so users can configure a future host while disabled. |
| I2P mixed mode and I2P PeX | `i2p_mixed_mode`, `i2p_pex_enabled` | bool, bool | Read/write, UI disables while I2P off. I2P PeX explicitly says “requires restart”; document that applying it may need a qBittorrent restart. |
| Proxy type | `proxy_type` | string enum: `None`, `SOCKS4`, `SOCKS5`, `HTTP` | Read/write. UI disables dependent inputs at `None`; SOCKS4 disallows auth, hostname lookup, RSS/general proxy toggles. Validate enum in Terraform; do not suppress dependent configured values from patch just because the selected type disables their UI controls. |
| Proxy host and port | `proxy_ip`, `proxy_port` | string; unsigned integer port (upstream converts toUInt; UI text input) | Read/write. UI enables host/port unless type is None. Add bounded port validation (0..65535 unless upstream/API evidence justifies narrower). |
| Proxy authentication and credentials | `proxy_auth_enabled`, `proxy_username`, `proxy_password` | bool, string, string | Read/write in WebUI; password field is explicitly documented as saved unencrypted. Treat password as secret in Terraform schema, never include it in the preferences allowlist/state/data source, and avoid logging request/response payloads. Since GET `/app/preferences` itself returns `proxy_password`, dropping it in client decode is required. Consider whether username is nonsecret and safe to include (it is not a password). |
| Proxy routing flags | `proxy_hostname_lookup`, `proxy_bittorrent`, `proxy_peer_connections`, `proxy_rss`, `proxy_misc` | all bool | Read/write. Mean DNS lookup via proxy, use proxy for BitTorrent, peer connections, RSS, and general purposes. UI dependencies described above are presentation only. |
| IP filter | `ip_filter_enabled`, `ip_filter_path`, `ip_filter_trackers` | bool, server-side path string, bool | Read/write. File path is on qBittorrent host, not Terraform runner. `ip_filter_path` input is UI-disabled when filter off but still saved. `ip_filter_trackers` controls applying the file to trackers. |
| Manually banned IPs | `banned_IPs` | newline-delimited string | Read/write; UI textarea. Server reads `session->bannedIPs().join('\n')`; set splits on newline with empty lines skipped. Preserve newlines and empty string. Server may normalize/drop invalid entries; read-after-write state should use returned value. |

The random-port preference `random_port` is deprecated upstream and is a separate legacy write path which sets port to zero. Do not expose it as a second managed setting; managing `listen_port` alongside a server-side `random_port=true` can cause drift/override. Existing docs already tell users to disable random-port settings before managing `listen_port`.

## BitTorrent

Markup: preferences.html lines 790–960; population lines 2745–2804; save lines 3190–3286. API read mapping: appcontroller.cpp lines 295–342. API patch mapping/enum translation: appcontroller.cpp lines 835–925. Numeric units and validation below come from UI labels and save-time checks; controller setters mostly consume `toInt()`/`toReal()` rather than duplicating those checks.

| UI control | Wire key(s) | Type / values / units | Read/write and notes |
|---|---|---|---|
| Encryption mode | `encryption` | integer enum `0` allow encryption, `1` require encryption, `2` disable encryption | Read/write. The HTML select is positional and the wire values match these values. |
| Anonymous mode | `anonymous_mode` | bool | Read/write. |
| Max active checking torrents | `max_active_checking_torrents` | integer, HTML number `min=-1`; `-1` is allowed upstream for unlimited | Read/write. UI saves directly without a client-side range check. Ensure validator allows documented sentinel. |
| Torrent queueing | `queueing_enabled` | bool | Read/write; when enabled UI exposes queue controls. |
| Maximum active downloads/uploads/torrents | `max_active_downloads`, `max_active_uploads`, `max_active_torrents` | integer, each `>= -1`; `-1` means unlimited | Read/write; UI submits these numeric keys only when queueing is enabled. For provider declarative config, send them whenever individually configured even when `queueing_enabled=false`, or users cannot preconfigure values and the server settings remain unmanaged. UI checkbox state is the queueing flag, not an implicit null/reset for child limits. |
| Count slow torrents in queue limits | `dont_count_slow_torrents` | bool | Wire polarity is easy to misread: UI checkbox says “Do not count slow torrents”; checked=true maps directly to true. Sent by UI only if queueing enabled. |
| Slow-torrent download/upload rate thresholds | `slow_torrent_dl_rate_threshold`, `slow_torrent_ul_rate_threshold` | integer KiB/s; UI requires >=1 | Read/write. HTML labels explicitly say KiB/s; API/session returns integer rates at this unit. |
| Slow-torrent inactivity timer | `slow_torrent_inactive_timer` | integer seconds; UI requires >=1 | Read/write. |
| Share ratio enabled and value | `max_ratio_enabled`, `max_ratio` | bool plus floating-point ratio (`0` or positive; disabled sends `-1`) | Read/write. A fractional ratio such as `1.5` must round-trip. Framework gap: `preferenceFields` only has integer number representation; add a Float64 kind and JSON conversion (and OpenAPI `number`/`double`) or a carefully justified decimal string API. Enabled false sends `max_ratio=-1`; enabled true uses exact value. Do not conflate valid 0.0 with null. |
| Total/inactive seeding duration enabled and values | `max_seeding_time_enabled`, `max_seeding_time`, `max_inactive_seeding_time_enabled`, `max_inactive_seeding_time` | bool plus integer minutes; enabled values >=0; disabled sends `-1` | Read/write. UI initializes disabled display fields to `1440`, but the persisted wire sentinel is -1. Honor exact configured zero. |
| Share limit mode (“Any/All”) | `share_limits_mode` | string enum `MatchAny` or `MatchAll` | Read/write. Radio is always selected in UI; load defaults from server. |
| Action when share limit reached | `max_ratio_act` | integer enum: `0` stop, `1` remove torrent, `2` enable super seeding, `3` remove torrent and files | Read/write. UI select values are non-display-ordered (super-seeding=2, remove content=3); save sends selected wire integer. Removing torrent/files is destructive when later triggered by a torrent reaching its limit; describe clearly. |
| Append trackers to new downloads | `add_trackers_enabled`, `add_trackers` | bool and newline-separated string | Read/write. String is disabled in UI while off, still read and posted. |
| Append trackers fetched from URL | `add_trackers_from_url_enabled`, `add_trackers_url` | bool and string URL | Read/write. |
| Fetched tracker list | `add_trackers_url_list` | newline-separated string | Read-only from preferences endpoint (`data` populated in appcontroller get; no matching `hasKey` setter in appcontroller set). Do not expose as configurable resource attribute. May be returned as a computed data-source attribute if the complete resource/data source contract requires it; provider currently models only preferences intended for both resource and data source, so simplest is omit until read-only-only modeling exists. |

Share-limit action/ratio are session behaviors. There is no special restart marker in this Options UI for these keys. Applying `max_ratio_act=1` or `3` authorizes future removal of torrents (and for 3, their content) on threshold; make this consequence explicit in Terraform docs.

## RSS

Markup: preferences.html lines 964–1020; population lines 2807–2815; save lines 3286–3294. API read mapping is appcontroller.cpp lines 389–395; setters are lines 1019–1034. RSS rules are a separate RSS API/modal surface, not a preferences key.

| UI control | Wire key(s) | Type / values / units | Read/write and notes |
|---|---|---|---|
| Enable feed fetching | `rss_processing_enabled` | bool | Read/write. |
| Feed refresh interval | `rss_refresh_interval` | integer minutes | Read/write. HTML shows min; application setter consumes int. |
| Same-host request delay | `rss_fetch_delay` | integer seconds | Read/write. Controller casts to 64-bit and `std::chrono::seconds`. |
| Max articles per feed | `rss_max_articles_per_feed` | integer count | Read/write. |
| Enable RSS auto-download | `rss_auto_downloading_enabled` | bool | Read/write; a preference only toggles processing and does not configure rules. |
| Download REPACK/PROPER episodes | `rss_download_repack_proper_episodes` | bool | Read/write. |
| RSS smart episode filter strings | `rss_smart_episode_filters` | newline-separated string list | Read/write; getter joins with newline, setter splits on newline. Preserve empty string and line boundaries. |
| Edit auto downloading rules… button | No `app/preferences` key. Calls `window.qBittorrent.Rss.openRssDownloader()` | Imperative UI action opening rule editor | Separate implementation scope: inspect RSS Web API rules endpoints and existing client/API support. Do not model as a Terraform preference or claim this inventory implements rule CRUD. |

The RSS page presents units but does not perform explicit range checks in the shown save block. Keep schema validation conservative and verify acceptable minimum/maximum against RSS setters/native definitions before enforcing bounds; integer types and unit declarations are established. These values apply immediately through RSS session setters; the UI has no restart warning.

## Suggested independent implementation slices

Keep each slice within shared `api/qbittorrent.yaml` / generated client / shared preference field table only if serialized sequentially; otherwise parallel work will conflict on those central files. A practical delegation plan is one integrator for API/schema/client/provider wiring and one worker per disjoint test/doc group. If multiple workers take attribute groups, have each return a list of fields, types, validators, and fixture expectations without editing shared central files; integrate their edits centrally.

1. **Connection protocol + numeric limits + I2P**: add protocol enum; four disabled-limit fields allowing `-1` or positive integers; I2P enable/host/port/mixed/PeX. Tests: schema enums/bounds and `-1`, read/write JSON patch includes explicitly configured disabled sentinel, false and zero retained, unknown/unconfigured values omitted, port range, I2P strings and booleans round-trip. Document I2P PeX restart caveat.
2. **Connection proxy + filters**: add all proxy fields except password to the public preference allowlist; decide and implement password handling separately as secret (or deliberately omit all credential fields while documenting the gap). Include proxy type enum, host/port, flags, username; IP filter path/toggles and banned list. Tests: enum rejection, all booleans including false, multiline banned IP round-trip/empty list, server path as string, API response containing `proxy_password` provably not serialized into state/data-source state or diagnostics/logs. UI's unencrypted password storage warrants prominent docs.
3. **BitTorrent privacy/queueing**: encryption enum, anonymous mode, checking max, queueing and all active/slow limits. Tests: enum values, min `-1` values, units in docs, bool polarity `dont_count_slow_torrents`, independent partial updates and sentinel fixture.
4. **BitTorrent share limits/trackers**: implement Float64 support for `max_ratio` end-to-end and enabled flags, seeding minute values/sentinels, mode/action enums, tracker text/URL. Tests: fractional ratio (e.g. 1.75) round-trips through JSON and Terraform state; enabled zero vs disabled -1; each action value; MatchAny/MatchAll; newline text; `add_trackers_url_list` never appears in writes. This slice should resolve the framework numeric design before API enum schema generation.
5. **RSS preference controls**: add interval/delay/article count and bool/string attributes. Tests: JSON unit-preserving round-trip, empty/multiline filters, all enable toggles and false values. Keep “Edit auto downloading rules” explicitly out of this slice; a separate rules task should inventory the RSS endpoints and build independent CRUD support if requested.

For all slices, the shared transport test should assert the POST form uses `json` with only managed keys and no unconfigured fields, and response decode should allowlist only public supported keys even though upstream returns additional values and secrets. Acceptance checks should use a qBittorrent 5.3.0 fixture/server, inspect resulting preferences, verify read-after-write normalization, and test import/relinquishment semantics. Do not make live server changes as part of the inventory.

## Provider baseline follow-up

Current implementation is table-driven: `internal/provider/preferences_resource.go` creates resource attributes from `preferenceFields`; `internal/provider/preferences_data_source.go` derives computed attributes; `internal/client/preferences.go` handles API allowlisting and patches; `api/qbittorrent.yaml` generates typed transport request/response models. Extend these consistently. The current supported subset is documented in `docs/preferences-by-tab.md` (Connection only `listen_port`, `upnp`; BitTorrent only `dht`, `pex`, `lsd`) and the current README states Speed is complete. This report intentionally does not change those artifacts or implement controls.
