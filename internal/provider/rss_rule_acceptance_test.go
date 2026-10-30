package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRSSRuleDoesNotFetchFeedsOrAddTorrents(t *testing.T) {
	setupAccEnv(t, false)
	c := accClient(t)
	const ruleName = "provider-acceptance-no-fetch"
	config := accProvider() + `
resource "qbittorrent_preferences" "rss_safety" {
  rss_processing_enabled = false
  rss_auto_downloading_enabled = false
}
resource "qbittorrent_rss_rule" "test" {
  rule_name = "provider-acceptance-no-fetch"
  enabled = false
  priority = 0
  use_regex = false
  must_contain = "no external feed"
  must_not_contain = ""
  episode_filter = ""
  affected_feeds = []
  ignore_days = 0
  smart_filter = false
  add_stopped = null
  content_layout = null
  torrent_params = {
    stopped = true
    content_layout = "Subfolder"
  }
  depends_on = [qbittorrent_preferences.rss_safety]
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			rules, err := c.RSSRules(context.Background())
			if err != nil {
				return err
			}
			if _, exists := rules[ruleName]; exists {
				return fmt.Errorf("RSS rule retained after destroy")
			}
			prefs, err := c.Preferences(context.Background())
			if err != nil {
				return err
			}
			if prefs.RssProcessingEnabled == nil || *prefs.RssProcessingEnabled || prefs.RssAutoDownloadingEnabled == nil || *prefs.RssAutoDownloadingEnabled {
				return fmt.Errorf("RSS processing or auto-downloading was enabled")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "id", ruleName),
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "enabled", "false"),
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "affected_feeds.#", "0"),
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "last_match", ""),
			)},
			{Config: config, PlanOnly: true},
			{ResourceName: "qbittorrent_rss_rule.test", ImportState: true, ImportStateId: ruleName, ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{"enabled", "priority", "use_regex", "must_contain", "must_not_contain", "episode_filter", "affected_feeds", "ignore_days", "smart_filter", "add_stopped", "content_layout", "save_path", "category", "torrent_params"}},
			{Config: config, PreConfig: func() {
				rules, err := c.RSSRules(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				rule, ok := rules[ruleName]
				if !ok {
					t.Fatal("acceptance rule disappeared before drift injection")
				}
				rule["mustContain"] = "server-side drift"
				if err := c.SetRSSRule(context.Background(), ruleName, rule); err != nil {
					t.Fatal(err)
				}
			}, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "must_contain", "no external feed"),
			)},
		},
	})
}
