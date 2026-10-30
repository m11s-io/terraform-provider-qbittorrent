# WebUI and Advanced preferences: inventory and implementation plan

Baseline: provider commit `a73a3a1` (release 0.3.0). Upstream source is the
user-pinned qBittorrent commit `9213075dd5f7b352208361c6a84c1c115efc898e`.
Use `git show <commit>:src/webui/www/private/views/preferences.html` and
`src/webui/api/appcontroller.cpp` from that exact commit: the checked-out
upstream HEAD/tag is different and should not be used as evidence for this
inventory.

Current provider coverage is summarized in `preferences-by-tab.md`: connection
has listen port and UPnP; Behavior, Speed, Downloads, BitTorrent, and WebUI
clientdata settings are partially or fully represented. This plan covers the
remaining controls in the **WebUI remote-control** tab and **Advanced** tab.
All listed controls use qBittorrent's `app/preferences` and `app/setPreferences`
unless noted. Keep Terraform attributes flat and snake_case in
`qbittorrent_preferences`; keep WebUI clientdata in its existing separate
resource and do not move app preferences into `clientdata/store`.

## WebUI — remote-control controls

The HTML has the following controls; wire keys and read/write behavior come
from `AppController::preferencesAction` and `setPreferencesAction`.

| Control | Wire key(s) | Wire type / semantics |
| --- | --- | --- |
| Bind IP address | `web_ui_address` | string; server listen address. |
| WebUI port | `web_ui_port` | integer (`quint16`); port 0 is accepted by the setter's conversion, though the page does not document a sentinel. Validate 1–65535 unless source validation establishes another supported value. |
| UPnP/NAT-PMP forwarding | `web_ui_upnp` | boolean. |
| HTTPS enabled | `use_https` | boolean. |
| HTTPS certificate and key paths | `web_ui_https_cert_path`, `web_ui_https_key_path` | server-side path strings. |
| Username | `web_ui_username` | string; setter rejects fewer than 3 characters or a colon. |
| Password | `web_ui_password` | write-only intent; setter hashes with PBKDF2 and requires at least 6 characters. The password is not returned by preferences read. Omit when not configured; never synthesize an empty password or read hashed data into Terraform. |
| API key value | `web_ui_api_key` | returned as string from preferences read. The UI displays it disabled and offers copy, generate/rotate, delete. Rotation and deletion are separate app API actions (`rotateAPIKeyAction` and `deleteAPIKeyAction`), not ordinary preference fields. Treat the value as sensitive. Decide whether it is safely representable as computed sensitive state; do not manage rotation/deletion as an implicit side effect of ordinary preference apply. |
| Bypass auth on localhost | `bypass_local_auth` | boolean; true weakens authentication. |
| Bypass auth for subnet whitelist | `bypass_auth_subnet_whitelist_enabled`, `bypass_auth_subnet_whitelist` | boolean plus newline-separated string; API setter also accepts commas. Parse/display subnet text without silently broadening ranges. |
| Ban after consecutive failures | `web_ui_max_auth_fail_count` | integer; HTML minimum 0. |
| Ban duration | `web_ui_ban_duration` | integer seconds; HTML minimum 1. |
| Session timeout | `web_ui_session_timeout` | integer seconds; HTML minimum 0. |
| Session count limit | `web_ui_sessions_count_limit` | integer; 0 means unlimited. |
| Alternative WebUI enabled and path | `alternative_webui_enabled`, `alternative_webui_path` | boolean and server-side directory path. |
| Clickjacking, CSRF, Secure cookie protection | `web_ui_clickjacking_protection_enabled`, `web_ui_csrf_protection_enabled`, `web_ui_secure_cookie_enabled` | booleans. Secure cookie requires HTTPS or localhost per the UI text. |
| Host header validation and allowed domains | `web_ui_host_header_validation_enabled`, `web_ui_domain_list` | boolean and semicolon-delimited domain string; `*` wildcard supported by UI. |
| Custom HTTP headers enabled and values | `web_ui_use_custom_http_headers_enabled`, `web_ui_custom_http_headers` | boolean and newline-delimited `Header: value` string. |
| Reverse proxy support and trusted proxies | `web_ui_reverse_proxy_enabled`, `web_ui_reverse_proxies_list` | boolean and semicolon-delimited addresses/subnets. Trusted entries enable using `X-Forwarded-For`; preserve exact supplied values. |
| Dynamic DNS enabled, provider, domain, username, password | `dyndns_enabled`, `dyndns_service`, `dyndns_domain`, `dyndns_username`, `dyndns_password` | boolean, integer enum `0` DynDNS / `1` NO-IP, strings. Password is returned by preferences read, so discard it from read models and data sources; manage only through write-only input with an update-version counter. |

The UI's API-key rotation/deletion buttons and DynDNS “Register” are actions,
not declarative preferences. Inventory those endpoints separately and do not
make a routine apply perform rotation, deletion, registration, or password
change just because a user manages other WebUI settings.

## Advanced — qBittorrent controls

| Control | Wire key(s) | Wire type / units / sentinel |
| --- | --- | --- |
| Resume data storage (restart required) | `resume_data_storage_type` | string enum `Legacy`, `SQLite`; SQLite is labeled experimental. |
| Torrent content removal mode | `torrent_content_remove_option` | string enum `Delete`, `MoveToTrash`; the former permanently deletes content. |
| Physical RAM working-set limit | `memory_working_set_limit` | integer MiB. |
| Network interface and optional bound address | `current_network_interface`, `current_interface_address` | strings; interface choices are discovered from host interfaces; empty interface/address means no explicit selection. `current_interface_address` is writable: appcontroller.cpp:1062 parses it as QHostAddress; invalid/empty values normalize to no address. Only `current_interface_name` is a read-only display companion. |
| Save resume data / statistics intervals | `save_resume_data_interval`, `save_statistics_interval` | integer minutes. |
| `.torrent` file size limit | `torrent_file_size_limit` | integer **bytes on the wire**, displayed as MiB in UI (divide/multiply by 1024²); preserve bytes in the Terraform attribute. |
| Confirm recheck; recheck completed torrents | `confirm_torrent_recheck`, `recheck_completed_torrents` | booleans. |
| Application instance name | `app_instance_name` | string. |
| Refresh interval | `refresh_interval` | integer milliseconds. |
| Resolve peer host names / countries | `resolve_peer_host_names`, `resolve_peer_countries` | booleans. |
| Confirm remove all tags / tracker from all torrents | `confirm_remove_all_tags`, `confirm_remove_tracker_from_all_torrents` | booleans (tracker key is a named constant in C++). |
| Reannounce on IP/port change | `reannounce_when_address_changed` | boolean. |
| Embedded tracker enabled, port, forwarding | `enable_embedded_tracker`, `embedded_tracker_port`, `embedded_tracker_port_forwarding` | boolean, integer port, boolean. |
| Mark-of-the-Web; ignore SSL errors | `mark_of_the_web`, `ignore_ssl_errors` | booleans; MOTW is platform-limited by UI; ignore SSL errors is security weakening and must retain the upstream default unless explicitly configured. |
| Python executable path | `python_executable_path` | path string; UI says may require restart; empty requests auto-detection. |
| Start session paused | `start_paused` | boolean. |
| Session shutdown timeout | `shutdown_timeout` | integer seconds, `-1` means unlimited; setter accepts only values >= -1. |

## Advanced — libtorrent controls

These map directly through the app preferences JSON. `appcontroller.cpp` gives
the wire key and C++ conversion; units/sentinels below follow the visible UI
labels. For numeric bounds not enforced by the HTML or setter, check the exact
qBittorrent/libtorrent version before adding Terraform validation; do not
invent limits.

| Control | Wire key | Type / units / visible enum or sentinel |
| --- | --- | --- |
| Bdecode depth / token limits | `bdecode_depth_limit`, `bdecode_token_limit` | integers. |
| Async I/O threads; hashing threads; file pool size | `async_io_threads`, `hashing_threads`, `file_pool_size` | integers; hashing row is conditionally shown. |
| Outstanding memory while checking torrents | `checking_memory_use` | integer MiB. |
| Disk cache; expiry; disk queue | `disk_cache`, `disk_cache_ttl`, `disk_queue_size` | integers; MiB, seconds, and **bytes on the wire** respectively (`disk_queue_size` is displayed as KiB). |
| Disk I/O type (restart required) | `disk_io_type` | integer enum 0 default, 1 memory-mapped, 2 POSIX-compliant, 3 simple pread/pwrite, 4 pread/pwrite; option 4 is conditionally available. |
| Disk read / write cache mode | `disk_io_read_mode`, `disk_io_write_mode` | integer enums: read 0 disable OS cache / 1 enable; write 0 disable / 1 enable / 2 write-through (conditional option). |
| Coalesce reads/writes; piece extent affinity; upload piece suggestions | `enable_coalesce_read_write`, `enable_piece_extent_affinity`, `enable_upload_suggestions` | booleans; coalesce row conditionally shown. |
| Send buffer watermark / low watermark / factor | `send_buffer_watermark`, `send_buffer_low_watermark`, `send_buffer_watermark_factor` | integers; KiB, KiB, percent. |
| Outgoing connections per second; outgoing connections while seeding | `connection_speed`, `seeding_outgoing_connections` | integer and boolean. |
| Socket send/receive buffers; listen backlog | `socket_send_buffer_size`, `socket_receive_buffer_size`, `socket_backlog_size` | integers; buffers are **bytes on the wire**, displayed as KiB; send/receive 0 means system default. |
| Outgoing port min/max | `outgoing_ports_min`, `outgoing_ports_max` | integers; 0 disables port restriction. |
| UPnP lease duration | `upnp_lease_duration` | integer seconds; 0 means permanent lease. |
| Peer DSCP | `peer_tos` | integer. |
| μTP/TCP mixed mode | `utp_tcp_mixed_mode` | integer enum 0 prefer TCP, 1 peer proportional (throttles TCP). |
| Hostname resolver cache TTL; IDN support | `hostname_cache_ttl`, `idn_support_enabled` | integer seconds and boolean. |
| Multiple connections from same IP / Peer ID | `enable_multi_connections_from_same_ip`, `enable_multi_connections_from_same_peer_id` | booleans; Peer ID row conditionally shown. |
| Validate HTTPS tracker certificates; SSRF mitigation; block privileged peer ports | `validate_https_tracker_certificate`, `ssrf_mitigation`, `block_peers_on_privileged_ports` | booleans. These are security controls; defaults must preserve the upstream secure behavior. |
| Upload slots behavior | `upload_slots_behavior` | integer enum 0 fixed slots, 1 upload-rate-based. |
| Upload choking algorithm | `upload_choking_algorithm` | integer enum 0 round-robin, 1 fastest upload, 2 anti-leech. |
| Announce to all trackers / tiers | `announce_to_all_trackers`, `announce_to_all_tiers` | booleans. |
| Announce IP / port (restart required) | `announce_ip`, `announce_port` | string address and integer port; empty IP means default; port 0 means listening port. |
| Maximum concurrent HTTP announces; stop tracker timeout | `max_concurrent_http_announces`, `stop_tracker_timeout` | integers; timeout seconds and 0 disables timeout. |
| Peer turnover / cutoff / interval | `peer_turnover`, `peer_turnover_cutoff`, `peer_turnover_interval` | integers; percent, percent, seconds. |
| Outstanding request queue / block requests | `request_queue_size`, `max_outstanding_block_requests` | integers. |
| DHT bootstrap nodes | `dht_bootstrap_nodes` | string; empty resets to default per UI. |
| WebTorrent STUN server | `webtorrent_stun_server` | string; row may be hidden when unsupported. |
| I2P inbound/outbound quantity, length, length variance | `i2p_inbound_quantity`, `i2p_outbound_quantity`, `i2p_inbound_length`, `i2p_outbound_length`, `i2p_inbound_length_variance`, `i2p_outbound_length_variance` | integers. UI bounds: quantities 1–16, lengths 0–7, variances -7–7. Conditional by libtorrent feature support. |

The UI hides selected rows based on linked libtorrent/version capability
(`rowHashingThreads`, disk I/O variants, coalesce, peer-ID connections,
WebTorrent STUN, I2P options). Upstream API preferences still contain keys for
some hidden controls. Since the provider cannot assume a server's linked
libtorrent feature set from a Terraform plan, avoid version-gating attributes
based only on the provider build. Prefer capability-aware read/round-trip
behavior, or document the server rejection/ignored-key behavior after checking
the relevant API and acceptance fixtures.

## Connection safety and state semantics

WebUI address, port, HTTPS enablement/certificates, username/password, and API
key operations can invalidate the provider's active HTTP session. A plan must
not silently weaken access controls to make itself reconnect. Keep normal
updates limited to configured fields. Do not retry by disabling authentication,
CSRF protection, host validation, HTTPS, or changing bind/port to a broader
address. For changes that predictably invalidate the current connection,
implement a bounded reconnect path using the target configuration and existing
credentials after the server applies the update; return an explicit diagnostic
with the applied/uncertain state if reconnection fails. Never print passwords,
API keys, cookies, or authorization headers. Document that a remote bind/port,
HTTPS, username/password, or API-key rotation may require provider endpoint or
credential updates in the same operation, and that API-key rotation/delete is
explicit rather than implied by other preferences.

For fields that are not returned or are action-only, model write-only values
without readback drift: use write-only arguments and nonsecret update-version counters; do not retain plaintext secrets in Terraform state,
never claim server-side verification, and avoid resending secrets on unrelated
updates. For ordinary optional fields, follow the established omission
semantics: only configured fields are sent, `null` relinquishes management, and
unknown/unmanaged server keys are not overwritten. Review plan/state output for
secrets and transitions from configured to null.

## Suggested multiple-Luna work split

Keep each batch within the existing `qbittorrent_preferences` resource/API
surface; share one agreed design for field metadata, typed wire conversion,
omission behavior, and acceptance coverage before parallel implementation.

1. **WebUI listener and authentication**: listener address/port, UPnP, HTTPS
   paths/flag, username/password and auth limits/whitelists. Add password
   sensitive/write-only behavior and connection-loss handling.
2. **WebUI controls and actions**: API key computed/action endpoint analysis,
   alt UI, security flags/domains/headers/trusted proxy, DynDNS. Keep generate,
   delete, and register as explicit actions if exposed at all; audit password
   sensitivity and security defaults.
3. **Advanced qBittorrent session controls**: resume mode/removal choice,
   memory/interface binding, save intervals/torrent-size/recheck/UI refresh,
   peer resolution/confirmations, tracker/Python/shutdown controls.
4. **Advanced libtorrent I/O/network controls**: decode/threads/cache/I/O,
   buffers/sockets/ports/DSCP/uTP/resolver/IDN/security/multi-connection.
5. **Advanced libtorrent tracker/peer controls**: choking, announce settings,
   peer turnover/request limits, DHT/STUN/I2P and conditional capability rows.

If multiple agents implement these batches, assign disjoint field groups and
have one integrator own shared schema/conversion helpers and the API contract.
Do not let two batches independently add the same fields or modify the common
preference field list without coordination.

## Verification plan for implementation

- Add unit tests for schema attribute names/types, enum/range validators,
  sensitive metadata, wire key mapping, integer widths, serialization, readback,
  and omission/null behavior. Include set-only password and action-only API key
  cases.
- Add focused client tests confirming `app/preferences` read and partial
  `app/setPreferences` writes preserve unrelated keys. Verify that non-secret
  updates do not resend write-only secrets.
- Add provider acceptance tests for import/read, create/update of one field per
  group, partial updates, null relinquishment, enum rejection, and API error
  diagnostics. Use a pinned qBittorrent 5.3.0 fixture from the exact upstream
  commit where possible; exercise connection-affecting cases only in a fixture
  capable of validating reconnect and clear failure behavior.
- Assert secrets are marked sensitive and absent from diagnostics, logs,
  examples, and rendered plans where the framework permits. Verify defaults do
  not disable HTTPS/auth/CSRF/host checks or other security protections.
- Cover server/version conditional fields by testing supported and unsupported
  qBittorrent/libtorrent combinations or by documenting the precise limitation;
  do not infer support solely from the UI's hidden rows.
- Regenerate the OpenAPI/client artifacts whenever fields or endpoints change;
  generated allowlist models must include every supported nonsecret wire key.
  Never hand-edit generated code.

## Source map and completeness notes

- UI labels, order, and visibility: `src/webui/www/private/views/preferences.html`
  at pinned upstream commit, `WebUITab` and `AdvancedTab`.
- Wire keys, JSON value types/conversions, setter validation, and API-key
  actions: `src/webui/api/appcontroller.cpp` at the same commit.
- WebUI API key read is present in preferences response; generation/deletion
  are separate actions. Password does not appear in preferences read.
- No controls in these two tabs were identified as clientdata-only. Existing
  behavior/display preferences in `clientdata/load` / `clientdata/store` are
  already documented and remain separate.
- Values not shown in a checkbox/select/input (for example exact upstream
  default numeric ranges) require checking the pinned preference definitions
  and linked libtorrent version before choosing validators. The UI and setter
  evidence above records exact visible enums, units, and sentinels; no extra
  range is assumed.
