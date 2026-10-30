# Preferences by WebUI tab

The OpenAPI specification keeps qBittorrent's flat wire format. Operations are
tagged by API area; preference properties carry `x-ui-tab` metadata. Terraform
attribute names remain flat and existing state is compatible.

## Behavior — application preferences

These fields belong to `qbittorrent_preferences` and its data source, using
`app/preferences` and `app/setPreferences`.

| UI control | Attribute | Values / units |
| --- | --- | --- |
| Language | `locale` | Upstream locale identifier, for example `en` |
| Confirm deletion | `confirm_torrent_deletion` | Boolean |
| Show external IP | `status_bar_external_ip` | Boolean |
| Save search tabs/results | `store_search_jobs`, `store_search_job_results` | Boolean |
| Enable log files | `file_log_enabled` | Boolean |
| Log save path | `file_log_path` | Server directory |
| Back up log after size | `file_log_backup_enabled`, `file_log_max_size` | Boolean and 1..1024000 KiB |
| Delete old logs | `file_log_delete_old`, `file_log_age`, `file_log_age_type` | Boolean, count 1..365, unit 0 days / 1 months / 2 years |
| Performance warnings | `performance_warning` | Boolean |

## Behavior — WebUI preferences

These fields belong to `qbittorrent_webui_preferences`, using `clientdata/load`
and `clientdata/store`; the provider adds the `qbt_` prefix. They are shared
server-side in `web_clientdata.json`, not isolated to a browser. Already-open
pages may require reloading to discard cached values.

| UI control | Attribute | Values / effective default |
| --- | --- | --- |
| Date format | `date_format` | `default` or one of the eight upstream formats documented on the resource |
| Color scheme | `color_scheme` | `auto`, `light`, `dark`; default `auto` |
| Display density | `display_density` | `default`, `compact`; default `default` |
| Alternating rows | `use_alt_row_colors` | Boolean; default false |
| Double-click downloading torrents | `dblclick_download` | String `0` no action, `1` start/stop; default `1` |
| Double-click completed torrents | `dblclick_complete` | String `0` no action, `1` start/stop; default `1` |
| Double-click filters | `dblclick_filter` | String `0` no action, `1` reset; default `1` |
| Hide zero filters | `hide_zero_status_filters` | Boolean; default false |
| Display torrent content/options | `add_new_torrent_dialog_enabled` | Boolean; default true |
| Full tracker announce URL | `full_url_tracker_column` | Boolean; default false |
| Optimized table rendering | `use_virtual_list` | Boolean; default true |

Only configured fields are managed. Terraform `null` relinquishes ownership
without resetting settings. Explicit `color_scheme = "auto"` and
`display_density = "default"` remove their stored overrides, matching the WebUI.
Missing keys read as effective defaults. Unknown keys are neither exposed nor
overwritten. Import ID is `application`; destroy removes state only. Use one
WebUI preferences resource per server.

## Other supported tabs

| Tab | Attributes in `qbittorrent_preferences` |
| --- | --- |
| Downloads | `save_path`, `temp_path`, `temp_path_enabled`, `auto_tmm_enabled` |
| Connection | `listen_port`, `upnp` |
| Speed | Normal/alternative limits, scheduler enablement, start/end hour and minute, days, uTP/overhead/LAN flags |
| BitTorrent | `dht`, `pex`, `lsd` |

All Behavior controls shown in the 5.3.0 WebUI and all Speed controls are covered.
Other tabs are partially covered; this remains a provider-specific API subset.
