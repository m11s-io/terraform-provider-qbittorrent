resource "qbittorrent_preferences" "application" {
  save_path         = "/downloads"
  temp_path         = "/downloads/incomplete"
  temp_path_enabled = true
  auto_tmm_enabled  = false
  dl_limit          = 0
  up_limit          = 1048576
  alt_dl_limit      = 10240
  alt_up_limit      = 10240

  # Times use the server's local time; configure each hour/minute pair together.
  scheduler_enabled  = false
  schedule_from_hour = 8
  schedule_from_min  = 0
  schedule_to_hour   = 20
  schedule_to_min    = 0
  scheduler_days     = 1 # Weekdays; 0 every day, 2 weekends, 3..9 Monday..Sunday.
  limit_utp_rate     = true
  limit_tcp_overhead = false
  limit_lan_peers    = true

  dht         = false
  pex         = false
  lsd         = false
  listen_port = 6881
  upnp        = false
}
