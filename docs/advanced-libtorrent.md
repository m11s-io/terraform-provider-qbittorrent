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
The async-I/O and hashing-thread setters store their inputs, but their getters
clamp the value to 1..1024. The provider applies those getter bounds to prevent
read-after-write normalization.
`disk_cache` retains negative values and has a universal native upper bound of
33554431 MiB. A 32-bit qBittorrent build instead caps it at 1536 MiB, so values
1537..33554431 can still normalize on a 32-bit server; the provider cannot infer
server bitness through this preferences contract.
When both outgoing ports are configured and non-zero, the minimum must not
exceed the maximum; a value that remains unknown until apply is checked then.

Enum values follow the pinned qBittorrent source at
`9213075dd5f7b352208361c6a84c1c115efc898e`:

- `disk_io_type`: 0 default, 1 memory mapped, 2 POSIX, 3 simple pread/pwrite,
  and upstream value 4 pread/pwrite where `LIBTORRENT_VERSION_NUM >= 20100`.
  The provider accepts 0..3 because the generic validator cannot query the
  target server's compile flags.
- `disk_io_read_mode`: 0 disable OS cache, 1 enable it.
- `disk_io_write_mode`: 0 disable OS cache, 1 enable it, and 2 write-through
  when `QBT_USES_LIBTORRENT2` is defined. The provider accepts 0..1 because the
  generic validator cannot query the target server's compile flags.
- `utp_tcp_mixed_mode`: 0 prefer TCP, 1 peer proportional.
- `upload_slots_behavior`: 0 fixed slots, 1 rate-based.
- `upload_choking_algorithm`: 0 round-robin, 1 fastest upload, 2 anti-leech.

The pinned acceptance image is qBittorrent 5.3.0 / Web API 2.16.2 at
`ghcr.io/linuxserver/qbittorrent@sha256:75ecbb1c89ab7b0a21c4bde40b4820e4f1649d2d56d83b91913422155e8d2a5f`.
Its `/app/buildInfo` reports bitness 64, platform `linux`, libtorrent `2.1.2.0`,
Boost `1.92.0`, Qt `6.12.0`, OpenSSL `4.1.0`, and zlib `1.3.2`. The preferences
API returns and stores these keys independently of whether the linked build maps
the stored value to an active libtorrent setting. The provider's capability
metadata checks whether a wire key is present in an API response; it does not
prove that a linked-library runtime feature is active. Synthetic missing-key
fixtures test only that API contract.

Pinned-source runtime conditions are:

| Preference | qBittorrent compile predicate for runtime application |
| --- | --- |
| `hashing_threads` | `QBT_USES_LIBTORRENT2` |
| `disk_cache`, `disk_cache_ttl`, `enable_coalesce_read_write` | `!QBT_USES_LIBTORRENT2` |
| `enable_multi_connections_from_same_peer_id` | `LIBTORRENT_VERSION_NUM >= 20013` |
| `webtorrent_stun_server` | `LIBTORRENT_VERSION_NUM >= 20100` |
| I2P quantities and lengths | `QBT_USES_LIBTORRENT2 && TORRENT_USE_I2P` |
| I2P length variance | I2P predicate above and `LIBTORRENT_VERSION_NUM >= 20012` |
| `disk_io_type = 4` | `LIBTORRENT_VERSION_NUM >= 20100` |
| `disk_io_write_mode = 2` | `QBT_USES_LIBTORRENT2` |

The linked `2.1.2.0` version satisfies the numeric version predicates for peer-ID
connections, STUN, I2P variance, and disk I/O type 4. The build-info endpoint does
not report compile definitions; `TORRENT_USE_I2P` is unverified and I2P runtime
behavior is not exercised by acceptance. qBittorrent applies disk cache, cache
expiry, and coalescing only in builds without `QBT_USES_LIBTORRENT2`, although the
API continues to store and return their preference values. The provider accepts
only common disk I/O enum values 0..3 and write modes 0..1 because per-server
compile definitions are not available to the generic preference validator.

An empty `dht_bootstrap_nodes` input clears the stored override, but
`getDHTBootstrapNodes()` returns the native default
`dht.libtorrent.org:25401, dht.transmissionbt.com:6881, router.bt.ouinet.work:6881`.
Read-after-write therefore reports a normalized value and state contains that
canonical default. Configure the canonical non-empty value for a stable plan.

Changing `disk_io_type`, `announce_ip`, or `announce_port` requires restarting
the qBittorrent application to affect runtime behavior. The provider stores and
reads back the preference and never restarts a server implicitly. Security
controls such as HTTPS tracker certificate validation and SSRF mitigation retain
upstream defaults unless explicitly configured.
