# Keep RSS processing disabled while creating rule configuration. This example
# deliberately has no feeds and does not enable the rule or auto-downloads.
resource "qbittorrent_rss_rule" "example" {
  rule_name       = "Example release rule"
  enabled         = false
  priority        = 0
  use_regex       = true
  must_contain    = "Example"
  must_not_contain = "sample"
  episode_filter  = "S[0-9]+E[0-9]+"
  affected_feeds   = []
  ignore_days     = 0
  smart_filter    = false

  # Null preserves qBittorrent's optional deprecated aliases.
  add_stopped    = null
  content_layout = null

  torrent_params = {
    category       = "tv"
    save_path      = "/srv/qbittorrent/downloads"
    stopped        = true
    content_layout = "Subfolder"
    download_limit = 0
  }
}
