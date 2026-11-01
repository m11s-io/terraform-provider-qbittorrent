resource "qbittorrent_preferences" "bittorrent_coverage" {
  encryption     = 0
  anonymous_mode = false

  max_active_checking_torrents   = -1
  queueing_enabled               = false
  max_active_downloads           = 0
  max_active_uploads             = -1
  max_active_torrents            = -1
  dont_count_slow_torrents       = false
  slow_torrent_dl_rate_threshold = 1
  slow_torrent_ul_rate_threshold = 1
  slow_torrent_inactive_timer    = 1

  max_ratio                 = 1.75
  max_seeding_time          = 0
  max_inactive_seeding_time = -1
  share_limits_mode         = "MatchAny"
  # 0 stops; 1 removes the torrent; 2 enables super seeding; 3 removes torrent and files.
  max_ratio_act = 0

  add_trackers_enabled          = false
  add_trackers                  = ""
  add_trackers_from_url_enabled = false
  add_trackers_url              = ""
}

data "qbittorrent_preferences" "bittorrent_coverage" {
  depends_on = [qbittorrent_preferences.bittorrent_coverage]
}
