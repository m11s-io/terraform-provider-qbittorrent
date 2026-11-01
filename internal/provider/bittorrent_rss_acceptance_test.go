package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccBitTorrentRSSPreferences(t *testing.T) {
	setupAccEnv(t, false)
	c := accClient(t)
	config := accProvider() + `
resource "qbittorrent_preferences" "bittorrent_rss" {
  encryption = 0
  anonymous_mode = false
  max_active_checking_torrents = -1
  queueing_enabled = false
  max_active_downloads = 0
  max_active_uploads = -1
  max_active_torrents = -1
  dont_count_slow_torrents = false
  slow_torrent_dl_rate_threshold = 1
  slow_torrent_ul_rate_threshold = 1
  slow_torrent_inactive_timer = 1
  max_ratio = 1.75
  max_seeding_time = 0
  max_inactive_seeding_time = -1
  share_limits_mode = "MatchAny"
  max_ratio_act = 0
  add_trackers_enabled = false
  add_trackers = ""
  add_trackers_from_url_enabled = false
  add_trackers_url = ""
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
data "qbittorrent_preferences" "bittorrent_rss" {
  depends_on = [qbittorrent_preferences.bittorrent_rss]
}
`
	zeroRatio := stringsReplaceOnce(config, "max_ratio = 1.75", "max_ratio = 0")
	disabledRatio := stringsReplaceOnce(config, "max_ratio = 1.75", "max_ratio = -1")
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: factories, CheckDestroy: func(*terraform.State) error {
		p, err := c.Preferences(context.Background())
		if err != nil {
			return err
		}
		if p.MaxRatio == nil || *p.MaxRatio != -1 {
			return fmt.Errorf("destroy reset or changed share ratio: %v", p.MaxRatio)
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("qbittorrent_preferences.bittorrent_rss", "max_ratio", "1.75"),
			resource.TestCheckResourceAttr("qbittorrent_preferences.bittorrent_rss", "max_seeding_time", "0"),
			resource.TestCheckResourceAttr("qbittorrent_preferences.bittorrent_rss", "rss_refresh_interval", "0"),
			resource.TestCheckResourceAttr("qbittorrent_preferences.bittorrent_rss", "rss_smart_episode_filters", "[group] Example\n\nS01E01\n"),
			resource.TestCheckResourceAttr("data.qbittorrent_preferences.bittorrent_rss", "max_ratio_enabled", "true"),
			resource.TestCheckResourceAttr("data.qbittorrent_preferences.bittorrent_rss", "rss_processing_enabled", "false"),
		)},
		{Config: config, PlanOnly: true},
		{Config: zeroRatio, Check: resource.TestCheckResourceAttr("qbittorrent_preferences.bittorrent_rss", "max_ratio", "0")},
		{Config: disabledRatio, Check: resource.TestCheckResourceAttr("data.qbittorrent_preferences.bittorrent_rss", "max_ratio_enabled", "false")},
	}})
}

func stringsReplaceOnce(s, old, replacement string) string {
	return strings.Replace(s, old, replacement, 1)
}
