package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestRSSRulesUseNamedFormFieldsAndTypedRuleAllowlist(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proxy/api/v2/rss/rules":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Weekly":{"enabled":true,"priority":4,"useRegex":false,"mustContain":"S01","mustNotContain":"sample","episodeFilter":"S\\d+E\\d+","affectedFeeds":["https://feed.invalid/rss"],"ignoreDays":2,"smartFilter":true,"lastMatch":"Fri, 30 Oct 2026 10:00:00 +0000","previouslyMatchedEpisodes":["S01E01"],"addPaused":null,"torrentContentLayout":"Original","savePath":"/downloads","assignedCategory":"tv","torrentParams":{"category":"tv","tags":["weekly"],"save_path":"/downloads","download_path":"/incomplete","use_download_path":false,"operating_mode":"Forced","seed_mode":true,"upload_limit":2000,"download_limit":1000,"ratio_limit":1.5,"seeding_time_limit":30,"inactive_seeding_time_limit":31,"share_limit_action":"Remove","share_limits_mode":"MatchAll","add_to_top_of_queue":true,"stopped":false,"stop_condition":"FilesChecked","content_layout":"Subfolder","use_auto_tmm":false,"unmanaged":"discard"},"unmanaged":"discard"}}`))
		case "/proxy/api/v2/rss/setRule":
			if r.Method != http.MethodPost {
				t.Errorf("setRule method = %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if len(r.Form) != 2 || r.Form.Get("ruleName") != "Weekly" {
				t.Errorf("setRule form = %v", r.Form)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("ruleDef")), &got); err != nil {
				t.Fatal(err)
			}
			if got["enabled"] != true || got["priority"] != float64(4) || got["mustContain"] != "S01" {
				t.Errorf("rule fields lost: %v", got)
			}
			if got["torrentContentLayout"] != "Original" {
				t.Errorf("legacy torrentContentLayout alias lost: %v", got)
			}
			if _, ok := got["unmanaged"]; ok {
				t.Errorf("unallowlisted field was sent: %v", got)
			}
			params, ok := got["torrentParams"].(map[string]any)
			if !ok || params["stopped"] != false || params["save_path"] != "/downloads" || params["download_path"] != "/incomplete" || params["use_download_path"] != false || params["operating_mode"] != "Forced" || params["seed_mode"] != true || params["upload_limit"] != float64(2000) || params["content_layout"] != "Subfolder" || params["download_limit"] != float64(1000) || params["ratio_limit"] != 1.5 || params["seeding_time_limit"] != float64(30) || params["inactive_seeding_time_limit"] != float64(31) || params["share_limit_action"] != "Remove" || params["share_limits_mode"] != "MatchAll" || params["stop_condition"] != "FilesChecked" || params["use_auto_tmm"] != false || params["add_to_top_of_queue"] != true {
				t.Errorf("native torrentParams lost: %v", got["torrentParams"])
			}
			if _, ok := params["contentLayout"]; ok {
				t.Errorf("non-native camelCase key sent: %v", params)
			}
			if _, ok := params["unmanaged"]; ok {
				t.Errorf("unallowlisted torrent param was sent: %v", params)
			}
		case "/proxy/api/v2/rss/removeRule":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if len(r.Form) != 1 || r.Form.Get("ruleName") != "Weekly" {
				t.Errorf("removeRule form = %v", r.Form)
			}
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
	}, false)

	rules, err := c.RSSRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rules["Weekly"]["priority"] != float64(4) || rules["Weekly"]["affectedFeeds"] == nil || rules["Weekly"]["torrentContentLayout"] != "Original" {
		t.Fatalf("rule response not decoded: %#v", rules)
	}
	if err := c.SetRSSRule(context.Background(), "Weekly", rules["Weekly"]); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveRSSRule(context.Background(), "Weekly"); err != nil {
		t.Fatal(err)
	}
}
