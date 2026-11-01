# Advanced qBittorrent session preferences

The `qbittorrent_preferences` resource owns the scalar Advanced session
preferences exposed by `app/preferences` and `app/setPreferences`. It keeps
partial ownership: only configured attributes are patched, and removing an
attribute relinquishes management without resetting the server value.

## Server-specific and read-only values

`current_network_interface` and `current_interface_address` are writable.
Interface names refer to the qBittorrent server's interfaces, never the
Terraform runner's interfaces. The API stores the supplied interface name and
resolves `current_interface_name` for display when that interface exists on the
server. The resolved name is a computed data-source output and is not sent in
patches. The server trims and canonicalizes valid interface addresses and
clears invalid or empty addresses, so read-after-write uses the server's
normalized value.

## Units and native numeric bounds

`torrent_file_size_limit` remains an exact byte count even though the WebUI
shows MiB. For example, 100 MiB is `104857600`. Save intervals use minutes,
`refresh_interval` uses milliseconds, and `shutdown_timeout` uses seconds.
Fields converted by the pinned API through `QJsonValue::toInt()` preserve the
native signed 32-bit range; the provider does not impose the WebUI's possible
minimums. `torrent_file_size_limit` uses the API's signed 64-bit conversion
without a narrower bound. `shutdown_timeout` accepts -1 through 2147483647,
where -1 means unlimited.

## Restart and platform behavior

Changing `resume_data_storage_type` requires a qBittorrent restart. Selecting
`SQLite` is experimental. `python_executable_path` may require a restart; an
empty value asks the server to detect Python automatically. The provider never
restarts a server during an ordinary preference apply. A dedicated acceptance
test switches the backend on its disposable instance, restarts that container,
checks persisted readback and no-op planning, then restores the original
backend and restarts again if needed.

`mark_of_the_web` is represented in the API but only has an effect on supported
macOS or Windows builds. It carries capability metadata so a server that omits
the key can be reported as unsupported before a configured write and represented
as null in data-source state.

The embedded tracker, interface binding, and SSL-error setting are ordinary
explicit inputs. Acceptance examples leave the embedded tracker disabled, do
not change bind interfaces, and keep SSL verification enabled.
