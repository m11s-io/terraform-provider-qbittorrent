resource "qbittorrent_preferences" "connection" {
  # Protocol: 0 TCP + µTP, 1 TCP, 2 µTP.
  bittorrent_protocol = 1

  # Use -1 to disable a limit; zero is rejected by qBittorrent's provider validator.
  max_connec               = -1
  max_connec_per_torrent   = 128
  max_uploads              = -1
  max_uploads_per_torrent  = 8

  # Keep I2P disabled while retaining its server-side configuration.
  i2p_enabled     = false
  i2p_address     = "127.0.0.1"
  i2p_port        = 7654
  i2p_mixed_mode  = false
  i2p_pex_enabled = false # qBittorrent marks I2P PeX as requiring a restart.

  # These configured dependents are retained while the proxy is disabled.
  proxy_type             = "None"
  proxy_ip               = "127.0.0.1"
  proxy_port             = 9050
  proxy_auth_enabled     = false
  proxy_username         = ""
  proxy_hostname_lookup  = false
  proxy_bittorrent        = false
  proxy_peer_connections = false
  proxy_rss              = false
  proxy_misc             = false

  # Filter settings are disabled; ip_filter_path, when configured, is server-side.
  ip_filter_enabled  = false
  ip_filter_trackers = false
  banned_ips         = ""

  # The Random button is not managed; listen_port is the declarative setting.
  upnp = false
}
