# Connection preferences implementation

The `qbittorrent_preferences` resource owns only configured Connection fields.
Connection limits accept a positive integer or `-1` to disable the limit; zero
is rejected because qBittorrent normalizes it to `-1`. `bittorrent_protocol`
accepts `0` for TCP and µTP, `1` for TCP only, or `2` for µTP only.

I2P host, port, mixed-mode and PeX values remain independently configurable
while I2P is disabled. qBittorrent labels I2P PeX as requiring a restart; the
provider does not restart the server. Proxy dependent values likewise remain
configurable while proxy type is `None` or authentication is disabled. Proxy
passwords are handled by the separate `qbittorrent_credentials` resource and
never enter the general preferences read model or data source. qBittorrent's
WebUI warns that proxy passwords are saved unencrypted.

`ip_filter_path` is a path on the qBittorrent server, not the Terraform runner.
`banned_ips` preserves the supplied raw newline-delimited text and maps to the
upstream wire key `banned_IPs`. qBittorrent skips empty lines and may normalize
or reject configured values; preference read-after-write diagnostics report a
different server-returned value instead of hiding the normalization.

The Connection tab's Random button is an action that fills the existing
`listen_port` input in the WebUI. It is not a persistent preference. The
deprecated `random_port` server key is not exposed, so managing a random-port
toggle cannot race with the existing `listen_port` field.

Connection acceptance coverage sets proxy, I2P, IP filtering and UPnP disabled.
It does not send test traffic through a proxy, enable router forwarding, or
depend on an external IP-filter file. The upstream source contract is pinned to
qBittorrent 5.3.0, commit
`9213075dd5f7b352208361c6a84c1c115efc898e`: `src/webui/api/appcontroller.cpp`
reads the fields around lines 240–294 and applies them around lines 730–833;
the UI's port range is in `src/webui/www/private/views/preferences.html`
around lines 535–690.
