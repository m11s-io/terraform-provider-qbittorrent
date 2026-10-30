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
			_, _ = w.Write([]byte(`{"Weekly":{"enabled":true,"priority":4,"useRegex":false,"mustContain":"S01","mustNotContain":"sample","episodeFilter":"S\\d+E\\d+","affectedFeeds":["https://feed.invalid/rss"],"ignoreDays":2,"smartFilter":true,"lastMatch":"Fri, 30 Oct 2026 10:00:00 +0000","previouslyMatchedEpisodes":["S01E01"],"addPaused":null,"contentLayout":null,"savePath":"/downloads","assignedCategory":"tv","torrentParams":{"category":"tv","savePath":"/downloads","stopped":false,"contentLayout":"Subfolder","downloadLimit":1000,"unmanaged":"discard"},"unmanaged":"discard"}}`))
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
			if _, ok := got["unmanaged"]; ok {
				t.Errorf("unallowlisted field was sent: %v", got)
			}
			params, ok := got["torrentParams"].(map[string]any)
			if !ok || params["stopped"] != false || params["contentLayout"] != "Subfolder" {
				t.Errorf("native torrentParams lost: %v", got["torrentParams"])
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
	if rules["Weekly"]["priority"] != float64(4) || rules["Weekly"]["affectedFeeds"] == nil {
		t.Fatalf("rule response not decoded: %#v", rules)
	}
	if err := c.SetRSSRule(context.Background(), "Weekly", rules["Weekly"]); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveRSSRule(context.Background(), "Weekly"); err != nil {
		t.Fatal(err)
	}
}
