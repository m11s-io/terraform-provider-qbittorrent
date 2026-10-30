resource "qbittorrent_preferences" "rss_coverage" {
  # Keep these disabled in offline examples: enabling them may fetch feeds or
  # launch downloads according to separately managed RSS rules.
  rss_processing_enabled = false
  rss_refresh_interval = 0
  rss_fetch_delay = 0
  rss_max_articles_per_feed = 0
  rss_auto_downloading_enabled = false
  rss_download_repack_proper_episodes = false
  rss_smart_episode_filters = <<-FILTERS
    [group] Example

    S01E01
  FILTERS
}

data "qbittorrent_preferences" "rss_coverage" {
  depends_on = [qbittorrent_preferences.rss_coverage]
}
