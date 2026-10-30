package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAdvancedLibtorrentLiteralWireUnitsAndRoundTrip(t *testing.T) {
	want := map[string]any{
		"disk_queue_size":              int64(104857600), // UI 102400 KiB, wire bytes.
		"socket_send_buffer_size":      int64(1048576),   // UI 1024 KiB, wire bytes.
		"socket_receive_buffer_size":   int64(2048),      // UI 2 KiB, wire bytes.
		"send_buffer_watermark":        int64(2048),      // KiB, no byte conversion.
		"send_buffer_low_watermark":    int64(512),
		"send_buffer_watermark_factor": int64(150), // percent.
		"peer_turnover":                int64(4),   // percent, as supplied.
		"hostname_cache_ttl":           int64(0),   // default/disabled sentinel.
		"upnp_lease_duration":          int64(0),   // permanent lease sentinel.
		"outgoing_ports_min":           int64(0), "outgoing_ports_max": int64(0),
		"announce_port": int64(0), "stop_tracker_timeout": int64(0),
		"announce_ip": "", "dht_bootstrap_nodes": "", "webtorrent_stun_server": "",
		"i2p_inbound_length_variance":        int64(-7), // permitted UI range includes negative values.
		"validate_https_tracker_certificate": true,
		"ssrf_mitigation":                    true,
	}
	fields := make(map[string]attr.Value, len(want))
	for k, v := range want {
		switch v := v.(type) {
		case string:
			fields[k] = types.StringValue(v)
		case bool:
			fields[k] = types.BoolValue(v)
		case int64:
			fields[k] = types.Int64Value(v)
		}
	}
	remote := map[string]any{}
	for key, value := range want {
		remote[key] = value
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
				t.Fatalf("patch has %d keys, want %d: %v", len(got), len(want), got)
			}
			for key, expected := range want {
				if number, ok := expected.(int64); ok {
					actual, ok := got[key].(float64)
					if !ok || actual != float64(number) {
						t.Errorf("wire %s = %#v, want %d", key, got[key], number)
					}
					continue
				}
				if got[key] != expected {
					t.Errorf("wire %s = %#v, want %#v", key, got[key], expected)
				}
				remote[key] = expected
			}
			return
		}
		_ = json.NewEncoder(w).Encode(remote)
	})
	r := &preferencesResource{client: c}
	s := testState(t, prefsSchema(), fields)
	var out resource.CreateResponse
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if out.Diagnostics.HasError() || writes != 1 {
		t.Fatalf("writes=%d diagnostics=%v", writes, out.Diagnostics)
	}
	var state types.Object
	if ds := out.State.Get(ctx, &state); ds.HasError() {
		t.Fatal(ds)
	}
	for key, expected := range fields {
		if got := state.Attributes()[key]; !expected.Equal(got) {
			t.Errorf("state %s = %v, want %v", key, got, expected)
		}
	}
}

func TestAdvancedLibtorrentFieldsHaveCompleteMetadataAndEnums(t *testing.T) {
	want := []string{
		"bdecode_depth_limit", "bdecode_token_limit", "async_io_threads", "hashing_threads", "file_pool_size", "checking_memory_use",
		"disk_cache", "disk_cache_ttl", "disk_queue_size", "disk_io_type", "disk_io_read_mode", "disk_io_write_mode",
		"enable_coalesce_read_write", "enable_piece_extent_affinity", "enable_upload_suggestions", "send_buffer_watermark",
		"send_buffer_low_watermark", "send_buffer_watermark_factor", "connection_speed", "seeding_outgoing_connections",
		"socket_send_buffer_size", "socket_receive_buffer_size", "socket_backlog_size", "outgoing_ports_min", "outgoing_ports_max",
		"upnp_lease_duration", "peer_tos", "utp_tcp_mixed_mode", "hostname_cache_ttl", "idn_support_enabled",
		"enable_multi_connections_from_same_ip", "enable_multi_connections_from_same_peer_id", "validate_https_tracker_certificate",
		"ssrf_mitigation", "block_peers_on_privileged_ports", "upload_slots_behavior", "upload_choking_algorithm",
		"announce_to_all_trackers", "announce_to_all_tiers", "announce_ip", "announce_port", "max_concurrent_http_announces",
		"stop_tracker_timeout", "peer_turnover", "peer_turnover_cutoff", "peer_turnover_interval", "request_queue_size",
		"max_outstanding_block_requests", "dht_bootstrap_nodes", "webtorrent_stun_server", "i2p_inbound_quantity",
		"i2p_outbound_quantity", "i2p_inbound_length", "i2p_outbound_length", "i2p_inbound_length_variance", "i2p_outbound_length_variance",
	}
	if len(advancedLibtorrentPreferenceFields) != len(want) {
		t.Fatalf("metadata fields=%d, want %d", len(advancedLibtorrentPreferenceFields), len(want))
	}
	for _, name := range want {
		f, ok := findPreferenceField(advancedLibtorrentPreferenceFields, name)
		if !ok {
			t.Errorf("missing metadata for %s", name)
			continue
		}
		if f.tab != "Advanced" || f.description == "" {
			t.Errorf("incomplete metadata for %s: %+v", name, f)
		}
		if _, ok := prefsSchema().Attributes[name]; !ok {
			t.Errorf("%s missing from Terraform schema", name)
		}
		if f.kind == preferenceInt && name != "disk_queue_size" {
			if f.integerMin == nil || f.integerMax == nil || *f.integerMin < -2147483648 || *f.integerMax > 2147483647 {
				t.Errorf("%s must fit the native QJsonValue::toInt signed 32-bit width: %+v", name, f)
			}
		}
	}
	for name, values := range map[string][]int64{
		"disk_io_type": {0, 1, 2, 3}, "disk_io_read_mode": {0, 1}, "disk_io_write_mode": {0, 1},
		"utp_tcp_mixed_mode": {0, 1}, "upload_slots_behavior": {0, 1}, "upload_choking_algorithm": {0, 1, 2},
	} {
		f, _ := findPreferenceField(advancedLibtorrentPreferenceFields, name)
		if len(f.integerEnums) != len(values) {
			t.Errorf("%s enum=%v, want %v", name, f.integerEnums, values)
		}
		for i := range values {
			if f.integerEnums[i] != values[i] {
				t.Errorf("%s enum=%v, want %v", name, f.integerEnums, values)
			}
		}
	}
}

func TestAdvancedLibtorrentCapabilityMetadataMarksConditionalRows(t *testing.T) {
	for _, name := range []string{"hashing_threads", "enable_coalesce_read_write", "enable_multi_connections_from_same_peer_id", "webtorrent_stun_server", "i2p_inbound_quantity", "i2p_outbound_length_variance"} {
		f, ok := findPreferenceField(advancedLibtorrentPreferenceFields, name)
		if !ok || f.capability == "" {
			t.Errorf("conditional %s lacks capability metadata: %+v", name, f)
		}
	}
}

func TestAdvancedLibtorrentNativeEnumValidators(t *testing.T) {
	valid := map[string][]int64{
		"disk_io_type":             {0, 1, 2, 3},
		"disk_io_read_mode":        {0, 1},
		"disk_io_write_mode":       {0, 1},
		"utp_tcp_mixed_mode":       {0, 1},
		"upload_slots_behavior":    {0, 1},
		"upload_choking_algorithm": {0, 1, 2},
	}
	s := prefsSchema()
	for name, accepted := range valid {
		attribute := s.Attributes[name].(interface{ Int64Validators() []validator.Int64 })
		allowed := map[int64]bool{}
		for _, value := range accepted {
			allowed[value] = true
		}
		for value := int64(-1); value <= 5; value++ {
			var out validator.Int64Response
			for _, check := range attribute.Int64Validators() {
				check.ValidateInt64(ctx, validator.Int64Request{Path: path.Root(name), ConfigValue: types.Int64Value(value)}, &out)
			}
			if out.Diagnostics.HasError() == allowed[value] {
				t.Errorf("%s=%d has errors=%v, accepted=%v", name, value, out.Diagnostics.HasError(), allowed[value])
			}
		}
	}
}

func TestAdvancedLibtorrentUnsupportedConditionalFieldStopsBeforeWrite(t *testing.T) {
	writes := 0
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "setPreferences") {
			writes++
			return
		}
		_, _ = w.Write([]byte(`{}`))
	})
	r := &preferencesResource{client: c}
	s := testState(t, prefsSchema(), map[string]attr.Value{"webtorrent_stun_server": types.StringValue("stun.example.test:3478")})
	var out resource.CreateResponse
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if writes != 0 || !out.Diagnostics.HasError() || !strings.Contains(fmt.Sprint(out.Diagnostics), "Unsupported preference capability") {
		t.Fatalf("writes=%d diagnostics=%v, want unsupported capability before patch", writes, out.Diagnostics)
	}
}

func TestAdvancedLibtorrentOutgoingPortsValidateOnlyKnownNonZeroRange(t *testing.T) {
	for _, tc := range []struct {
		name     string
		min, max attr.Value
		invalid  bool
	}{
		{name: "ordered", min: types.Int64Value(1000), max: types.Int64Value(2000)},
		{name: "both disabled", min: types.Int64Value(0), max: types.Int64Value(0)},
		{name: "unknown defers", min: types.Int64Value(5000), max: types.Int64Unknown()},
		{name: "inverted", min: types.Int64Value(3000), max: types.Int64Value(2000), invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]attr.Value{"outgoing_ports_min": tc.min, "outgoing_ports_max": tc.max}
			var d diag.Diagnostics
			validateAdvancedLibtorrentOutgoingPorts(values, &d)
			if d.HasError() != tc.invalid {
				t.Fatalf("diagnostics=%v, invalid=%v", d, tc.invalid)
			}
		})
	}
}
