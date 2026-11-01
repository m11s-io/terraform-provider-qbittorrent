package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

func TestAccRSSRuleDoesNotFetchFeedsOrAddTorrents(t *testing.T) {
	setupAccEnv(t, false)
	c := accClient(t)
	no := false
	if err := c.SetPreferences(context.Background(), client.Preferences{RssProcessingEnabled: &no, RssAutoDownloadingEnabled: &no}); err != nil {
		t.Fatal(err)
	}
	baselineTorrents := accRSSRuleTorrentHashes(t)
	baselineFeeds := accRSSFeedInventory(t)
	const ruleName = "provider-acceptance-no-fetch"
	checkNoRSSSideEffects := func(*terraform.State) error {
		if current := accRSSRuleTorrentHashes(t); !reflect.DeepEqual(baselineTorrents, current) {
			return fmt.Errorf("RSS rule CRUD changed torrent hashes: before %v after %v", baselineTorrents, current)
		}
		if current := accRSSFeedInventory(t); baselineFeeds != current {
			return fmt.Errorf("RSS rule CRUD changed feed inventory")
		}
		return nil
	}
	checkTagRoundTrip := func(*terraform.State) error {
		rules, err := c.RSSRules(context.Background())
		if err != nil {
			return err
		}
		params, ok := rules[ruleName]["torrentParams"].(map[string]any)
		if !ok {
			return fmt.Errorf("RSS rule has no torrentParams after write")
		}
		tags, ok := params["tags"].([]any)
		if !ok || len(tags) != 2 {
			return fmt.Errorf("native tag round trip should contain two distinct tags, got %#v", params["tags"])
		}
		seen := map[string]bool{}
		for _, raw := range tags {
			tag, ok := raw.(string)
			if !ok {
				return fmt.Errorf("native tag is not a string: %#v", raw)
			}
			seen[tag] = true
		}
		if len(seen) != 2 || !seen["z"] || !seen["a"] {
			return fmt.Errorf("native tag round trip lost a configured tag: %#v", tags)
		}
		return nil
	}
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
    tags = ["z", "a", "z"]
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
			return checkNoRSSSideEffects(nil)
		},
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeTestCheckFunc(checkNoRSSSideEffects, checkTagRoundTrip,
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "id", ruleName),
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "enabled", "false"),
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "affected_feeds.#", "0"),
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "last_match", ""),
			)},
			{Config: config, PlanOnly: true},
			{ResourceName: "qbittorrent_rss_rule.test", ImportState: true, ImportStateId: ruleName, ImportStateVerify: true,
				Check:                   resource.ComposeTestCheckFunc(checkNoRSSSideEffects),
				ImportStateVerifyIgnore: []string{"enabled", "priority", "use_regex", "must_contain", "must_not_contain", "episode_filter", "affected_feeds", "ignore_days", "smart_filter", "last_match", "previously_matched_episodes", "add_stopped", "content_layout", "save_path", "category", "torrent_params"}},
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
			}, PlanOnly: true, ExpectNonEmptyPlan: true, Check: resource.ComposeTestCheckFunc(checkNoRSSSideEffects)},
			{Config: config, Check: resource.ComposeTestCheckFunc(checkNoRSSSideEffects,
				resource.TestCheckResourceAttr("qbittorrent_rss_rule.test", "must_contain", "no external feed"),
			)},
		},
	})
}

func accJSONGET(t *testing.T, endpoint string) []byte {
	t.Helper()
	base, err := url.Parse(os.Getenv("QBT_ACC_URL"))
	if err != nil {
		t.Fatal("invalid acceptance URL")
	}
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal("invalid acceptance endpoint")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/v2/" + strings.TrimLeft(endpointURL.Path, "/")
	base.RawQuery = endpointURL.RawQuery
	request, err := http.NewRequest(http.MethodGet, base.String(), nil)
	if err != nil {
		t.Fatal("cannot build acceptance read request")
	}
	request.Header.Set("Authorization", "Bearer "+os.Getenv("QBT_ACC_API_KEY"))
	request.Header.Set("Origin", base.Scheme+"://"+base.Host)
	request.Header.Set("Referer", base.Scheme+"://"+base.Host+"/")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal("acceptance read request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("acceptance read returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		t.Fatal("cannot read acceptance response")
	}
	return body
}

func accRSSRuleTorrentHashes(t *testing.T) map[string]struct{} {
	t.Helper()
	var torrents []struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(accJSONGET(t, "torrents/info"), &torrents); err != nil {
		t.Fatal("invalid torrent inventory response")
	}
	hashes := make(map[string]struct{}, len(torrents))
	for _, torrent := range torrents {
		hashes[torrent.Hash] = struct{}{}
	}
	return hashes
}

func accRSSFeedInventory(t *testing.T) string {
	t.Helper()
	endpoint := "rss/items?withData=false"
	var inventory any
	if err := json.Unmarshal(accJSONGET(t, endpoint), &inventory); err != nil {
		t.Fatal("invalid RSS feed inventory response")
	}
	canonical, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal("cannot normalize RSS feed inventory")
	}
	return string(canonical)
}
