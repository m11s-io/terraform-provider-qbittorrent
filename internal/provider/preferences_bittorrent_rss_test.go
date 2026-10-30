package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

func TestBitTorrentAndRSSPreferenceMetadata(t *testing.T) {
	if err := validatePreferenceFields(append(append([]preferenceField{}, bittorrentPreferenceFields...), rssPreferenceFields...)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"encryption", "anonymous_mode", "max_active_checking_torrents", "queueing_enabled",
		"max_active_downloads", "max_active_uploads", "max_active_torrents", "dont_count_slow_torrents",
		"slow_torrent_dl_rate_threshold", "slow_torrent_ul_rate_threshold", "slow_torrent_inactive_timer",
		"max_ratio", "max_seeding_time", "max_inactive_seeding_time", "share_limits_mode", "max_ratio_act",
		"add_trackers_enabled", "add_trackers", "add_trackers_from_url_enabled", "add_trackers_url", "add_trackers_url_list",
		"rss_processing_enabled", "rss_refresh_interval", "rss_fetch_delay", "rss_max_articles_per_feed",
		"rss_auto_downloading_enabled", "rss_download_repack_proper_episodes", "rss_smart_episode_filters",
	} {
		if _, ok := findPreferenceField(append(append([]preferenceField{}, bittorrentPreferenceFields...), rssPreferenceFields...), name); !ok {
			t.Errorf("missing preference metadata for %s", name)
		}
	}
	for _, name := range []string{"max_ratio_enabled", "max_seeding_time_enabled", "max_inactive_seeding_time_enabled", "add_trackers_url_list"} {
		f, ok := findPreferenceField(bittorrentPreferenceFields, name)
		if !ok || !f.readOnly {
			t.Errorf("%s must be modeled as read-only data-source output", name)
		}
	}
}

func TestBitTorrentPreferenceValidators(t *testing.T) {
	s := prefsSchema()
	for name, cases := range map[string]struct {
		valid, invalid []int64
	}{
		"encryption":                     {[]int64{0, 1, 2}, []int64{-1, 3}},
		"max_active_checking_torrents":   {[]int64{-2147483648, -1, 0, 2, 2147483647}, []int64{-2147483649, 2147483648}},
		"max_active_downloads":           {[]int64{-1, 0, 2}, []int64{-2}},
		"max_active_uploads":             {[]int64{-1, 0, 2}, []int64{-2}},
		"max_active_torrents":            {[]int64{-1, 0, 2}, []int64{-2}},
		"slow_torrent_dl_rate_threshold": {[]int64{-2147483648, -1, 0, 1, 1024, 2147483647}, []int64{-2147483649, 2147483648}},
		"slow_torrent_ul_rate_threshold": {[]int64{-2147483648, -1, 0, 1, 1024, 2147483647}, []int64{-2147483649, 2147483648}},
		"slow_torrent_inactive_timer":    {[]int64{-2147483648, -1, 0, 1, 300, 2147483647}, []int64{-2147483649, 2147483648}},
		"max_seeding_time":               {[]int64{-1, 0, 120}, []int64{-2}},
		"max_inactive_seeding_time":      {[]int64{-1, 0, 120}, []int64{-2}},
		"max_ratio_act":                  {[]int64{0, 1, 2, 3}, []int64{-1, 4}},
	} {
		a := s.Attributes[name].(interface{ Int64Validators() []validator.Int64 })
		for _, n := range cases.valid {
			if got := validateInt64(t, a.Int64Validators(), name, n); got {
				t.Errorf("%s rejected valid %d", name, n)
			}
		}
		for _, n := range cases.invalid {
			if got := validateInt64(t, a.Int64Validators(), name, n); !got {
				t.Errorf("%s accepted invalid %d", name, n)
			}
		}
	}
	for _, value := range []string{"MatchAny", "MatchAll"} {
		if got := validateString(t, s.Attributes["share_limits_mode"], "share_limits_mode", value); got {
			t.Errorf("share_limits_mode rejected %q", value)
		}
	}
	for _, value := range []string{"MatchNone", ""} {
		if got := validateString(t, s.Attributes["share_limits_mode"], "share_limits_mode", value); !got {
			t.Errorf("share_limits_mode accepted %q", value)
		}
	}
	if got := validateFloat(t, s.Attributes["max_ratio"], "max_ratio", 1.75); got {
		t.Error("max_ratio rejected fractional ratio 1.75")
	}
	if got := validateFloat(t, s.Attributes["max_ratio"], "max_ratio", -1); got {
		t.Error("max_ratio rejected disabled sentinel -1")
	}
	if got := validateFloat(t, s.Attributes["max_ratio"], "max_ratio", -1.5); !got {
		t.Error("max_ratio accepted non-sentinel negative ratio")
	}
}

func TestRSSPreferenceBoundariesAndUnits(t *testing.T) {
	s := prefsSchema()
	for name, cases := range map[string]struct {
		valid, invalid []int64
	}{
		"rss_refresh_interval":      {[]int64{-1, 0, 30}, []int64{1 << 31}},
		"rss_fetch_delay":           {[]int64{-1, 0, 30, 1<<63 - 1}, nil},
		"rss_max_articles_per_feed": {[]int64{0, 1, 100}, []int64{-1, 1 << 31}},
	} {
		a := s.Attributes[name].(interface{ Int64Validators() []validator.Int64 })
		for _, n := range cases.valid {
			if got := validateInt64(t, a.Int64Validators(), name, n); got {
				t.Errorf("%s rejected valid %d", name, n)
			}
		}
		for _, n := range cases.invalid {
			if got := validateInt64(t, a.Int64Validators(), name, n); !got {
				t.Errorf("%s accepted invalid %d", name, n)
			}
		}
	}
}

func TestBitTorrentAndRSSPreferenceWireRoundTrip(t *testing.T) {
	filters := "[group] Show\n\nS01E01"
	trackers := "udp://tracker.example:80/announce\nhttps://tracker.example/announce"
	want := map[string]any{
		"encryption": float64(2), "anonymous_mode": true,
		"max_active_checking_torrents": float64(-1), "queueing_enabled": false,
		"max_active_downloads": float64(0), "max_active_uploads": float64(4), "max_active_torrents": float64(-1),
		"dont_count_slow_torrents": true, "slow_torrent_dl_rate_threshold": float64(1),
		"slow_torrent_ul_rate_threshold": float64(1024), "slow_torrent_inactive_timer": float64(30),
		"max_ratio": float64(1.75), "max_seeding_time": float64(0), "max_inactive_seeding_time": float64(-1),
		"share_limits_mode": "MatchAll", "max_ratio_act": float64(0),
		"add_trackers_enabled": false, "add_trackers": trackers,
		"add_trackers_from_url_enabled": false, "add_trackers_url": "https://trackers.example/list.txt",
		"rss_processing_enabled": false, "rss_refresh_interval": float64(0), "rss_fetch_delay": float64(0),
		"rss_max_articles_per_feed": float64(0), "rss_auto_downloading_enabled": false,
		"rss_download_repack_proper_episodes": false, "rss_smart_episode_filters": filters,
	}
	fields := map[string]attr.Value{}
	for k, v := range want {
		switch v := v.(type) {
		case bool:
			fields[k] = types.BoolValue(v)
		case float64:
			if k == "max_ratio" {
				fields[k] = types.Float64Value(v)
			} else {
				fields[k] = types.Int64Value(int64(v))
			}
		case string:
			fields[k] = types.StringValue(v)
		}
	}
	writes := 0
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "setPreferences") {
			writes++
			_ = r.ParseForm()
			var got map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("json")), &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("patch keys differ: got %v", got)
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("patch %s: got %#v want %#v", k, got[k], v)
				}
			}
			if _, ok := got["max_ratio_enabled"]; ok {
				t.Error("derived max_ratio_enabled was patched")
			}
			if _, ok := got["add_trackers_url_list"]; ok {
				t.Error("fetched tracker list was patched")
			}
			return
		}
		response := map[string]any{}
		for k, v := range want {
			response[k] = v
		}
		response["max_ratio_enabled"] = true
		response["max_seeding_time_enabled"] = true
		response["max_inactive_seeding_time_enabled"] = false
		response["add_trackers_url_list"] = trackers
		_ = json.NewEncoder(w).Encode(response)
	})
	r := &preferencesResource{client: c}
	s := testState(t, prefsSchema(), fields)
	var out resource.CreateResponse
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if out.Diagnostics.HasError() || writes != 1 {
		t.Fatalf("writes=%d diagnostics=%v", writes, out.Diagnostics)
	}
	var obj types.Object
	out.State.Get(ctx, &obj)
	for name, value := range fields {
		if !value.Equal(obj.Attributes()[name]) {
			t.Errorf("state %s: got %v want %v", name, obj.Attributes()[name], value)
		}
	}
	if !strings.Contains(filters, "\n\n") {
		t.Fatal("fixture must include a blank filter line")
	}
}

func TestRSSDistinctUnitsAndEmptyFilterRoundTrip(t *testing.T) {
	want := map[string]any{
		"rss_processing_enabled":    false,
		"rss_refresh_interval":      float64(17),
		"rss_fetch_delay":           float64(43),
		"rss_smart_episode_filters": "",
	}
	fields := map[string]attr.Value{
		"rss_processing_enabled":    types.BoolValue(false),
		"rss_refresh_interval":      types.Int64Value(17),
		"rss_fetch_delay":           types.Int64Value(43),
		"rss_smart_episode_filters": types.StringValue(""),
	}
	writes := 0
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "setPreferences") {
			writes++
			_ = r.ParseForm()
			var got map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("json")), &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("patch keys differ: got %v", got)
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("patch %s: got %#v want %#v", k, got[k], v)
				}
			}
			return
		}
		_ = json.NewEncoder(w).Encode(want)
	})
	r := &preferencesResource{client: c}
	s := testState(t, prefsSchema(), fields)
	var out resource.CreateResponse
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if out.Diagnostics.HasError() || writes != 1 {
		t.Fatalf("writes=%d diagnostics=%v", writes, out.Diagnostics)
	}
	var obj types.Object
	out.State.Get(ctx, &obj)
	for name, value := range fields {
		if !value.Equal(obj.Attributes()[name]) {
			t.Errorf("state %s: got %v want %v", name, obj.Attributes()[name], value)
		}
	}
}

func TestShareRatioWireAndStateRoundTrip(t *testing.T) {
	for _, ratio := range []float64{1.75, 0, -1} {
		t.Run(fmt.Sprintf("%g", ratio), func(t *testing.T) {
			writes := 0
			c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "setPreferences") {
					writes++
					_ = r.ParseForm()
					var got map[string]any
					if err := json.Unmarshal([]byte(r.Form.Get("json")), &got); err != nil {
						t.Fatal(err)
					}
					if len(got) != 1 || got["max_ratio"] != ratio {
						t.Fatalf("patch = %#v, want max_ratio=%v only", got, ratio)
					}
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"max_ratio": ratio, "max_ratio_enabled": ratio >= 0,
				})
			})
			r := &preferencesResource{client: c}
			s := testState(t, prefsSchema(), map[string]attr.Value{"max_ratio": types.Float64Value(ratio)})
			var out resource.CreateResponse
			r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
			if out.Diagnostics.HasError() || writes != 1 {
				t.Fatalf("writes=%d diagnostics=%v", writes, out.Diagnostics)
			}
			var obj types.Object
			out.State.Get(ctx, &obj)
			if got := obj.Attributes()["max_ratio"]; !types.Float64Value(ratio).Equal(got) {
				t.Fatalf("state max_ratio = %v, want %v", got, ratio)
			}
		})
	}
}

func TestShareLimitPatchPrecedence(t *testing.T) {
	fields := append(append([]preferenceField{}, bittorrentPreferenceFields...), rssPreferenceFields...)
	for _, order := range [][]string{{"max_ratio", "max_ratio_enabled"}, {"max_ratio_enabled", "max_ratio"}} {
		patch := map[string]any{}
		for _, name := range order {
			value := any(float64(1.75))
			if name == "max_ratio_enabled" {
				value = false
			}
			wire, patchValue, ok := preferencePatchEntry(fields, name, value)
			if ok {
				patch[wire] = patchValue
			}
		}
		if len(patch) != 1 || patch["max_ratio"] != float64(1.75) {
			t.Errorf("managed share-limit patch changed by field iteration order %v: %#v", order, patch)
		}
	}
	for _, name := range []string{"max_ratio_enabled", "max_seeding_time_enabled", "max_inactive_seeding_time_enabled"} {
		if _, exists := prefsSchema().Attributes[name]; exists {
			t.Errorf("derived flag %s appeared in resource schema", name)
		}
	}
}

func TestLegacyShareLimitFlagCannotCompeteWithNumericSetter(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "setPreferences") {
			_ = r.ParseForm()
			var got map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("json")), &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got["max_ratio"] != 1.75 {
				t.Errorf("legacy setter sent competing derived flag: %#v", got)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	enabled := false
	ratio := 1.75
	if err := c.SetPreferences(ctx, client.Preferences{MaxRatioEnabled: &enabled, MaxRatio: &ratio}); err != nil {
		t.Fatal(err)
	}
}

func validateInt64(t *testing.T, validators []validator.Int64, name string, n int64) bool {
	t.Helper()
	var resp validator.Int64Response
	for _, v := range validators {
		v.ValidateInt64(ctx, validator.Int64Request{Path: path.Root(name), ConfigValue: types.Int64Value(n)}, &resp)
	}
	return resp.Diagnostics.HasError()
}

func validateFloat(t *testing.T, attribute any, name string, n float64) bool {
	t.Helper()
	a := attribute.(interface{ Float64Validators() []validator.Float64 })
	var resp validator.Float64Response
	for _, v := range a.Float64Validators() {
		v.ValidateFloat64(ctx, validator.Float64Request{Path: path.Root(name), ConfigValue: types.Float64Value(n)}, &resp)
	}
	return resp.Diagnostics.HasError()
}

func validateString(t *testing.T, attribute any, name, value string) bool {
	t.Helper()
	a := attribute.(interface{ StringValidators() []validator.String })
	var resp validator.StringResponse
	for _, v := range a.StringValidators() {
		v.ValidateString(ctx, validator.StringRequest{Path: path.Root(name), ConfigValue: types.StringValue(value)}, &resp)
	}
	return resp.Diagnostics.HasError()
}
