package provider

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAdvancedSessionPreferenceMetadata(t *testing.T) {
	wantKinds := map[string]preferenceKind{
		"resume_data_storage_type":                 preferenceString,
		"torrent_content_remove_option":            preferenceString,
		"memory_working_set_limit":                 preferenceInt,
		"current_network_interface":                preferenceString,
		"current_interface_address":                preferenceString,
		"current_interface_name":                   preferenceString,
		"save_resume_data_interval":                preferenceInt,
		"save_statistics_interval":                 preferenceInt,
		"torrent_file_size_limit":                  preferenceInt,
		"confirm_torrent_recheck":                  preferenceBool,
		"recheck_completed_torrents":               preferenceBool,
		"app_instance_name":                        preferenceString,
		"refresh_interval":                         preferenceInt,
		"resolve_peer_host_names":                  preferenceBool,
		"resolve_peer_countries":                   preferenceBool,
		"confirm_remove_all_tags":                  preferenceBool,
		"confirm_remove_tracker_from_all_torrents": preferenceBool,
		"reannounce_when_address_changed":          preferenceBool,
		"enable_embedded_tracker":                  preferenceBool,
		"embedded_tracker_port":                    preferenceInt,
		"embedded_tracker_port_forwarding":         preferenceBool,
		"mark_of_the_web":                          preferenceBool,
		"ignore_ssl_errors":                        preferenceBool,
		"python_executable_path":                   preferenceString,
		"start_paused":                             preferenceBool,
		"shutdown_timeout":                         preferenceInt,
	}
	if len(advancedSessionPreferenceFields) != len(wantKinds) {
		t.Fatalf("got %d advanced session fields, want %d", len(advancedSessionPreferenceFields), len(wantKinds))
	}
	for name, kind := range wantKinds {
		field, ok := findPreferenceField(advancedSessionPreferenceFields, name)
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if field.kind != kind || field.wireKey != name || field.tab != "Advanced" || field.description == "" {
			t.Errorf("%s has incomplete or incorrect metadata: %+v", name, field)
		}
	}
	resolved, _ := findPreferenceField(advancedSessionPreferenceFields, "current_interface_name")
	if !resolved.readOnly || resolved.excludeFromDataSource {
		t.Errorf("resolved interface name must be read-only data-source output: %+v", resolved)
	}
	for _, name := range []string{"current_network_interface", "current_interface_address"} {
		field, _ := findPreferenceField(advancedSessionPreferenceFields, name)
		if field.readOnly {
			t.Errorf("%s must be writable", name)
		}
	}
	for _, name := range []string{"resume_data_storage_type", "python_executable_path"} {
		field, _ := findPreferenceField(advancedSessionPreferenceFields, name)
		if !strings.Contains(field.description, "restart") {
			t.Errorf("%s must document restart behavior: %q", name, field.description)
		}
	}
	motw, _ := findPreferenceField(advancedSessionPreferenceFields, "mark_of_the_web")
	if motw.capability == "" {
		t.Error("mark_of_the_web must declare its platform-dependent capability")
	}
	if err := validatePreferenceFields(advancedSessionPreferenceFields); err != nil {
		t.Fatal(err)
	}
}

func TestAdvancedSessionNativeRangesAndSentinels(t *testing.T) {
	for _, name := range []string{"memory_working_set_limit", "save_resume_data_interval", "save_statistics_interval", "refresh_interval", "embedded_tracker_port"} {
		field, ok := findPreferenceField(advancedSessionPreferenceFields, name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if field.integerMin == nil || *field.integerMin != math.MinInt32 || field.integerMax == nil || *field.integerMax != math.MaxInt32 {
			t.Errorf("%s must preserve native signed-int32 range, got min=%v max=%v", name, field.integerMin, field.integerMax)
		}
	}
	shutdown, _ := findPreferenceField(advancedSessionPreferenceFields, "shutdown_timeout")
	if shutdown.integerMin == nil || *shutdown.integerMin != -1 || shutdown.integerMax == nil || *shutdown.integerMax != math.MaxInt32 || len(shutdown.integerSentinels) != 1 || shutdown.integerSentinels[0] != -1 {
		t.Fatalf("shutdown timeout must accept -1..INT_MAX with -1 sentinel, got %+v", shutdown)
	}
	for _, value := range []int64{-2, -1, 0, math.MaxInt32, math.MaxInt32 + 1} {
		var response validator.Int64Response
		(preferenceIntValidator{field: shutdown}).ValidateInt64(ctx, validator.Int64Request{Path: path.Root("shutdown_timeout"), ConfigValue: types.Int64Value(value)}, &response)
		want := value >= -1 && value <= math.MaxInt32
		if got := !response.Diagnostics.HasError(); got != want {
			t.Errorf("shutdown_timeout %d valid=%v want %v", value, got, want)
		}
	}
	limit, _ := findPreferenceField(advancedSessionPreferenceFields, "torrent_file_size_limit")
	if limit.integerMin != nil || limit.integerMax != nil {
		t.Fatalf("torrent_file_size_limit must preserve native int64 range, got %+v", limit)
	}
	wire, value, include := preferencePatchEntry(advancedSessionPreferenceFields, "torrent_file_size_limit", int64(104857600))
	encoded, err := json.Marshal(map[string]any{wire: value})
	if err != nil {
		t.Fatal(err)
	}
	if !include || string(encoded) != `{"torrent_file_size_limit":104857600}` {
		t.Fatalf("100 MiB must be sent as exact byte count, got included=%v JSON=%s", include, encoded)
	}
}
