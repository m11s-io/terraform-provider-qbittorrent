package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestConnectionPreferenceMetadata(t *testing.T) {
	want := map[string]string{
		"bittorrent_protocol": "Connection", "max_connec": "Connection",
		"max_connec_per_torrent": "Connection", "max_uploads": "Connection",
		"max_uploads_per_torrent": "Connection", "i2p_enabled": "Connection",
		"i2p_address": "Connection", "i2p_port": "Connection",
		"i2p_mixed_mode": "Connection", "i2p_pex_enabled": "Connection",
		"proxy_type": "Connection", "proxy_ip": "Connection", "proxy_port": "Connection",
		"proxy_auth_enabled": "Connection", "proxy_username": "Connection",
		"proxy_hostname_lookup": "Connection", "proxy_bittorrent": "Connection",
		"proxy_peer_connections": "Connection", "proxy_rss": "Connection", "proxy_misc": "Connection",
		"ip_filter_enabled": "Connection", "ip_filter_path": "Connection",
		"ip_filter_trackers": "Connection", "banned_ips": "Connection",
	}
	if len(connectionPreferenceFields) != len(want) {
		t.Fatalf("got %d connection fields, want %d", len(connectionPreferenceFields), len(want))
	}
	for _, f := range connectionPreferenceFields {
		if tab, ok := want[f.name]; !ok || f.tab != tab {
			t.Errorf("unexpected field metadata: %+v", f)
		}
		delete(want, f.name)
	}
	if len(want) != 0 {
		t.Errorf("missing connection fields: %v", want)
	}
}

func TestConnectionEnumsSentinelsAndPorts(t *testing.T) {
	protocol, ok := findPreferenceField(connectionPreferenceFields, "bittorrent_protocol")
	if !ok || !equalInt64s(protocol.integerEnums, []int64{0, 1, 2}) {
		t.Fatalf("protocol enum = %+v", protocol.integerEnums)
	}
	for _, tc := range []struct {
		value int64
		valid bool
	}{{0, true}, {1, true}, {2, true}, {3, false}} {
		var resp validator.Int64Response
		(preferenceIntValidator{field: protocol}).ValidateInt64(context.Background(), validator.Int64Request{
			Path: path.Root(protocol.name), ConfigValue: types.Int64Value(tc.value),
		}, &resp)
		if got := !resp.Diagnostics.HasError(); got != tc.valid {
			t.Errorf("protocol=%d valid=%v want %v", tc.value, got, tc.valid)
		}
	}
	for _, name := range []string{"max_connec", "max_connec_per_torrent", "max_uploads", "max_uploads_per_torrent"} {
		f, ok := findPreferenceField(connectionPreferenceFields, name)
		if !ok || !equalInt64s(f.integerSentinels, []int64{-1}) || f.integerMin == nil || *f.integerMin != 1 {
			t.Errorf("%s must permit positive values and -1 only: %+v", name, f)
		}
	}
	for _, name := range []string{"i2p_port", "proxy_port"} {
		f, ok := findPreferenceField(connectionPreferenceFields, name)
		if !ok || f.integerMin == nil || *f.integerMin != 0 || f.integerMax == nil || *f.integerMax != 65535 {
			t.Errorf("%s must use its 0..65535 port range: %+v", name, f)
		}
	}
	proxy, ok := findPreferenceField(connectionPreferenceFields, "proxy_type")
	if !ok || !equalStrings(proxy.stringEnums, []string{"None", "SOCKS4", "SOCKS5", "HTTP"}) {
		t.Fatalf("proxy type enum = %+v", proxy.stringEnums)
	}
	for _, tc := range []struct {
		value string
		valid bool
	}{{"None", true}, {"SOCKS4", true}, {"SOCKS5", true}, {"HTTP", true}, {"HTTPS", false}} {
		var resp validator.StringResponse
		(stringChoiceValidator{allowed: proxy.stringEnums}).ValidateString(context.Background(), validator.StringRequest{
			Path: path.Root(proxy.name), ConfigValue: types.StringValue(tc.value),
		}, &resp)
		if got := !resp.Diagnostics.HasError(); got != tc.valid {
			t.Errorf("proxy_type=%s valid=%v want %v", tc.value, got, tc.valid)
		}
	}
	banned, ok := findPreferenceField(connectionPreferenceFields, "banned_ips")
	if !ok || banned.wireKey != "banned_IPs" {
		t.Fatalf("banned IPs wire key = %q", banned.wireKey)
	}
}

func TestConnectionStringFieldsPreserveRawTextAndConfiguredDisabledValues(t *testing.T) {
	const raw = "192.0.2.1\n\n198.51.100.2\n"
	wire, value, include := preferencePatchEntry(connectionPreferenceFields, "banned_ips", raw)
	if !include || wire != "banned_IPs" || value != raw {
		t.Fatalf("banned IP patch = (%q, %q, %v)", wire, value, include)
	}
	for _, name := range []string{"proxy_username", "proxy_ip", "ip_filter_path"} {
		wire, value, include = preferencePatchEntry(connectionPreferenceFields, name, "configured while disabled")
		if !include || wire != name || value != "configured while disabled" {
			t.Errorf("configured dependent %s was suppressed: %q %#v %v", name, wire, value, include)
		}
	}
	for _, name := range []string{"proxy_auth_enabled", "proxy_hostname_lookup", "proxy_bittorrent", "proxy_peer_connections", "proxy_rss", "proxy_misc", "ip_filter_enabled", "ip_filter_trackers"} {
		wire, value, include = preferencePatchEntry(connectionPreferenceFields, name, false)
		if !include || wire != name || value != false {
			t.Errorf("explicit false %s was suppressed: %q %#v %v", name, wire, value, include)
		}
	}
	wire, value, include = preferencePatchEntry(connectionPreferenceFields, "banned_ips", "")
	if !include || wire != "banned_IPs" || value != "" {
		t.Errorf("explicit empty banned IP list was suppressed: %q %#v %v", wire, value, include)
	}
}

func TestConnectionNumericValidatorsRejectZeroLimitsAndOutOfRangePorts(t *testing.T) {
	for _, name := range []string{"max_connec", "max_connec_per_torrent", "max_uploads", "max_uploads_per_torrent", "i2p_port", "proxy_port"} {
		f, ok := findPreferenceField(connectionPreferenceFields, name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		tests := []struct {
			value int64
			valid bool
		}{{0, name == "i2p_port" || name == "proxy_port"}, {65535, true}}
		if name == "i2p_port" || name == "proxy_port" {
			tests = append(tests, struct {
				value int64
				valid bool
			}{65536, false}, struct {
				value int64
				valid bool
			}{-1, false})
		} else {
			tests = append(tests, struct {
				value int64
				valid bool
			}{-1, true}, struct {
				value int64
				valid bool
			}{2147483647, true}, struct {
				value int64
				valid bool
			}{2147483648, false})
		}
		for _, tc := range tests {
			var resp validator.Int64Response
			(preferenceIntValidator{field: f}).ValidateInt64(context.Background(), validator.Int64Request{
				Path: path.Root(name), ConfigValue: types.Int64Value(tc.value),
			}, &resp)
			if got := !resp.Diagnostics.HasError(); got != tc.valid {
				t.Errorf("%s=%d valid=%v want %v (%v)", name, tc.value, got, tc.valid, resp.Diagnostics)
			}
		}
	}
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
