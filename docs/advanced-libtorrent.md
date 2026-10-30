# Advanced libtorrent preferences

The attributes in `qbittorrent_preferences` use the qBittorrent Web API wire
units. `disk_queue_size` and `socket_send_buffer_size` / `socket_receive_buffer_size`
are bytes, although the WebUI displays those values in KiB. Send-buffer watermarks
are already KiB; the watermark factor and peer turnover values are percentages.
UPnP lease duration and hostname cache TTL are seconds. Zero retains the UI's
stated meanings, including system-default socket buffers, disabled outgoing-port
restrictions, permanent UPnP lease, listening-port announce, and empty/default
resolver or tracker values. I2P length variance accepts negative values in the
UI's -7..7 range.

Integer fields whose qBittorrent setter converts with `QJsonValue::toInt()` are
bounded to signed 32-bit values. `disk_queue_size` is passed to `toLongLong()`
and retains signed 64-bit wire width. Numeric values are otherwise not assigned
invented minima or maxima: native setter behavior is the source for validation.
For example, peer-turnover setters store signed integer inputs; the WebUI's
0..100 and non-negative checks do not become provider-only restrictions.
When both outgoing ports are configured and non-zero, the minimum must not
exceed the maximum; a value that remains unknown until apply is checked then.

Enum values follow the pinned qBittorrent source at
`9213075dd5f7b352208361c6a84c1c115efc898e`:

- `disk_io_type`: 0 default, 1 memory mapped, 2 POSIX, 3 simple pread/pwrite,
  and upstream value 4 pread/pwrite where `LIBTORRENT_VERSION_NUM >= 20100`.
  The provider accepts 0..3 until the linked libtorrent build can be checked.
- `disk_io_read_mode`: 0 disable OS cache, 1 enable it.
- `disk_io_write_mode`: 0 disable OS cache, 1 enable it, and 2 write-through
  when `QBT_USES_LIBTORRENT2` is defined. The provider accepts 0..1 until the
  linked build can be checked.
- `utp_tcp_mixed_mode`: 0 prefer TCP, 1 peer proportional.
- `upload_slots_behavior`: 0 fixed slots, 1 rate-based.
- `upload_choking_algorithm`: 0 round-robin, 1 fastest upload, 2 anti-leech.

The API response includes controls even when the WebUI hides a row. Fields with
known linked-build or platform variations carry capability metadata so an absent
wire key becomes null in the data source and a configured apply is refused before
writing. The pinned acceptance image is qBittorrent 5.3.0 / Web API 2.16.2 at
`ghcr.io/linuxserver/qbittorrent@sha256:75ecbb1c89ab7b0a21c4bde40b4820e4f1649d2d56d83b91913422155e8d2a5f`.
Its acceptance fixture does not inspect every optional runtime feature; conditional
capabilities remain explicitly unexercised rather than inferred from the UI.

Changing `disk_io_type`, `announce_ip`, or `announce_port` requires restarting
the qBittorrent application to affect runtime behavior. The provider stores and
reads back the preference and never restarts a server implicitly. Security
controls such as HTTPS tracker certificate validation and SSRF mitigation retain
upstream defaults unless explicitly configured.
