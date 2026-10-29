resource "qbittorrent_preferences" "application" {
  save_path         = "/downloads"
  temp_path         = "/downloads/incomplete"
  temp_path_enabled = true
  auto_tmm_enabled  = false
  dl_limit          = 0
  up_limit          = 1048576
  dht               = false
  pex               = false
  lsd               = false
  listen_port       = 6881
  upnp              = false
}
