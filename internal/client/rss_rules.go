package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

// RSSRule contains the qBittorrent 5.3 RSS rule fields owned by this provider.
// Runtime history is read and supplied back on writes to prevent replacing a
// rule from resetting its match history.
type RSSRule map[string]any

type RSSRules map[string]RSSRule

var rssRuleFields = map[string]struct{}{
	"enabled": {}, "priority": {}, "useRegex": {}, "mustContain": {}, "mustNotContain": {},
	"episodeFilter": {}, "affectedFeeds": {}, "lastMatch": {}, "ignoreDays": {}, "smartFilter": {},
	"previouslyMatchedEpisodes": {}, "addPaused": {}, "torrentContentLayout": {}, "savePath": {},
	"assignedCategory": {}, "torrentParams": {},
}

// These are the public nonsecret AddTorrentParams fields used by RSS rules.
// SSL private-key material and API-only file selection fields are excluded.
var rssTorrentParamFields = map[string]struct{}{
	"category": {}, "tags": {}, "save_path": {}, "download_path": {}, "use_download_path": {},
	"operating_mode": {}, "seed_mode": {}, "upload_limit": {}, "download_limit": {},
	"ratio_limit": {}, "seeding_time_limit": {}, "inactive_seeding_time_limit": {},
	"share_limit_action": {}, "share_limits_mode": {}, "add_to_top_of_queue": {}, "stopped": {},
	"stop_condition": {}, "content_layout": {}, "use_auto_tmm": {},
}

func (c *Client) RSSRules(ctx context.Context) (RSSRules, error) {
	b, err := c.do(ctx, "rss/rules", true, func(ctx context.Context) (*http.Response, error) {
		return c.api.GetRSSRules(ctx)
	})
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := decodeObject(b, &raw); err != nil {
		return nil, err
	}
	rules := make(RSSRules, len(raw))
	for name, encoded := range raw {
		var rule RSSRule
		if err := json.Unmarshal(encoded, &rule); err != nil {
			return nil, errors.New("qBittorrent RSS rule response is invalid")
		}
		rules[name] = filterRSSRule(rule)
	}
	return rules, nil
}

func (c *Client) SetRSSRule(ctx context.Context, name string, rule RSSRule) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("RSS rule name must not be empty")
	}
	filtered := filterRSSRule(rule)
	b, err := json.Marshal(filtered)
	if err != nil {
		return errors.New("cannot encode RSS rule")
	}
	form := url.Values{"ruleName": {name}, "ruleDef": {string(b)}}
	_, err = c.do(ctx, "rss/setRule", false, func(ctx context.Context) (*http.Response, error) {
		return c.api.SetRSSRuleWithBody(ctx, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	})
	return err
}

func (c *Client) RemoveRSSRule(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("RSS rule name must not be empty")
	}
	_, err := c.do(ctx, "rss/removeRule", false, func(ctx context.Context) (*http.Response, error) {
		return c.api.RemoveRSSRuleWithFormdataBody(ctx, openapi.RemoveRSSRuleFormdataRequestBody{RuleName: name})
	})
	return err
}

func filterRSSRule(rule RSSRule) RSSRule {
	filtered := make(RSSRule, len(rssRuleFields))
	for key, value := range rule {
		if _, ok := rssRuleFields[key]; !ok {
			continue
		}
		if key == "torrentParams" {
			params, ok := value.(map[string]any)
			if !ok {
				continue
			}
			filteredParams := make(map[string]any, len(rssTorrentParamFields))
			for param, paramValue := range params {
				if _, ok := rssTorrentParamFields[param]; ok {
					filteredParams[param] = paramValue
				}
			}
			filtered[key] = filteredParams
			continue
		}
		filtered[key] = value
	}
	return filtered
}
