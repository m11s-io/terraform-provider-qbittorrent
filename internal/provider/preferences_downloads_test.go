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
)

func TestDownloadsPreferencesPatchAndRefresh(t *testing.T) {
	want := map[string]any{
		"torrent_content_layout": "Subfolder", "add_to_top_of_queue": false,
		"add_stopped_enabled": true, "torrent_stop_condition": "FilesChecked",
		"merge_trackers": false, "auto_delete_mode": float64(0), "preallocate_all": true,
		"incomplete_files_ext": false, "use_unwanted_folder": true,
		"torrent_changed_tmm_enabled": false, "save_path_changed_tmm_enabled": true,
		"category_changed_tmm_enabled": false, "use_category_paths_in_manual_mode": true,
		"torrent_files_backup_enabled": false, "torrent_files_backup_dir": "",
		"torrent_files_finished_backup_dir_enabled": true, "torrent_files_finished_backup_dir": "/archive/finished",
		"remove_torrent_file_backup":  false,
		"excluded_file_names_enabled": true, "excluded_file_names": "*.tmp\n*.part\n",
		"mail_notification_enabled": false, "mail_notification_sender": "from@example.test",
		"mail_notification_email": "one@example.test,two@example.test;ops@example.test",
		"mail_notification_smtp":  "smtp.example.test:587", "mail_notification_encryption_type": "STARTTLS",
		"mail_notification_auth_enabled": true, "mail_notification_username": "mailer",
		"autorun_on_torrent_added_enabled": true, "autorun_on_torrent_added_program": "/usr/bin/echo added",
		"autorun_enabled": false, "autorun_program": "/usr/bin/echo finished",
	}
	fields := make(map[string]attr.Value, len(want))
	for k, v := range want {
		switch v := v.(type) {
		case string:
			fields[k] = types.StringValue(v)
		case bool:
			fields[k] = types.BoolValue(v)
		case float64:
			fields[k] = types.Int64Value(int64(v))
		}
	}
	remote := map[string]any{"unmanaged_future_setting": "untouched"}
	writes := 0
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "setPreferences") {
			writes++
			r.ParseForm()
			var got map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("json")), &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("unexpected patch keys: %v", got)
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("patch %s: got %#v, want %#v", k, got[k], v)
				}
				remote[k] = v
			}
			return
		}
		json.NewEncoder(w).Encode(remote)
	})
	r := &preferencesResource{client: c}
	s := testState(t, prefsSchema(), fields)
	var out resource.CreateResponse
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if out.Diagnostics.HasError() || writes != 1 {
		t.Fatalf("writes=%d diagnostics=%v", writes, out.Diagnostics)
	}
	if remote["unmanaged_future_setting"] != "untouched" {
		t.Fatalf("unmanaged setting changed: %v", remote)
	}
	var obj types.Object
	if ds := out.State.Get(ctx, &obj); ds.HasError() {
		t.Fatal(ds)
	}
	for k, v := range fields {
		if got := obj.Attributes()[k]; !v.Equal(got) {
			t.Errorf("state %s: got %v, want %v", k, got, v)
		}
	}
}

func TestDownloadsEnumAndAutoDeleteValidation(t *testing.T) {
	s := prefsSchema()
	for _, tc := range []struct {
		name  string
		valid []string
	}{
		{"torrent_content_layout", []string{"Original", "Subfolder", "NoSubfolder"}},
		{"torrent_stop_condition", []string{"None", "MetadataReceived", "FilesChecked"}},
		{"mail_notification_encryption_type", []string{"None", "STARTTLS", "SMTPS"}},
	} {
		a, ok := s.Attributes[tc.name].(interface{ StringValidators() []validator.String })
		if !ok {
			t.Fatalf("%s has no string validators", tc.name)
		}
		valid := map[string]bool{}
		for _, v := range tc.valid {
			valid[v] = true
		}
		for _, v := range append(append([]string{}, tc.valid...), "invalid") {
			var out validator.StringResponse
			for _, check := range a.StringValidators() {
				check.ValidateString(ctx, validator.StringRequest{Path: path.Root(tc.name), ConfigValue: types.StringValue(v)}, &out)
			}
			if out.Diagnostics.HasError() == valid[v] {
				t.Errorf("%s=%q diagnostics: %v", tc.name, v, out.Diagnostics)
			}
		}
	}
	a, ok := s.Attributes["auto_delete_mode"].(interface{ Int64Validators() []validator.Int64 })
	if !ok {
		t.Fatal("auto_delete_mode has no integer validators")
	}
	for _, v := range []int64{-1, 0, 1, 2, 3} {
		var out validator.Int64Response
		for _, check := range a.Int64Validators() {
			check.ValidateInt64(ctx, validator.Int64Request{Path: path.Root("auto_delete_mode"), ConfigValue: types.Int64Value(v)}, &out)
		}
		if out.Diagnostics.HasError() != (v < 0 || v > 2) {
			t.Errorf("auto_delete_mode=%d diagnostics: %v", v, out.Diagnostics)
		}
	}
}

func TestDownloadsCommandNormalizationAndNullRelinquishment(t *testing.T) {
	remote := map[string]any{"autorun_program": "  /usr/bin/echo normalized  ", "save_path": "/kept"}
	writes := 0
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "setPreferences") {
			writes++
			r.ParseForm()
			var patch map[string]any
			json.Unmarshal([]byte(r.Form.Get("json")), &patch)
			if len(patch) != 1 || patch["autorun_program"] != "  /usr/bin/echo normalized  " {
				t.Fatalf("unexpected command patch: %v", patch)
			}
			remote["autorun_program"] = strings.TrimSpace(patch["autorun_program"].(string))
			return
		}
		json.NewEncoder(w).Encode(remote)
	})
	r := &preferencesResource{client: c}
	s := testState(t, prefsSchema(), map[string]attr.Value{"autorun_program": types.StringValue("  /usr/bin/echo normalized  ")})
	var out resource.CreateResponse
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if !out.Diagnostics.HasError() || !strings.Contains(fmt.Sprint(out.Diagnostics), "autorun_program") {
		t.Fatalf("expected normalization diagnostic, got %v", out.Diagnostics)
	}
	if writes != 1 {
		t.Fatalf("writes=%d", writes)
	}

	// Clearing the configured value to null relinquishes ownership and sends no patch.
	relinquish := testState(t, prefsSchema(), map[string]attr.Value{"autorun_program": types.StringNull()})
	var update resource.UpdateResponse
	r.Update(ctx, resource.UpdateRequest{Plan: tfsdk.Plan{Schema: relinquish.Schema, Raw: relinquish.Raw}}, &update)
	if update.Diagnostics.HasError() || writes != 1 {
		t.Fatalf("null relinquishment wrote or failed: writes=%d diagnostics=%v", writes, update.Diagnostics)
	}
}

func TestDownloadsCommandTemplatesSensitiveAndDataSourceExcluded(t *testing.T) {
	for _, name := range []string{"autorun_on_torrent_added_program", "autorun_program"} {
		a := prefsSchema().Attributes[name].(interface{ IsSensitive() bool })
		if !a.IsSensitive() {
			t.Errorf("%s is not sensitive", name)
		}
	}
	for _, f := range downloadsPreferenceFields {
		if strings.HasSuffix(f.name, "_program") && !f.excludeFromDataSource {
			t.Errorf("command template %s is exposed by the general data source", f.name)
		}
		switch f.name {
		case "save_path", "temp_path", "temp_path_enabled", "auto_tmm_enabled", "scan_dirs", "mail_notification_password":
			t.Errorf("Downloads field %q belongs to another preference owner", f.name)
		}
	}
	if err := validatePreferenceFields(allPreferenceFields()); err != nil {
		t.Fatalf("Downloads metadata has duplicate or invalid ownership: %v", err)
	}
	all := allPreferenceFields()
	for _, name := range []string{"save_path", "temp_path", "temp_path_enabled", "auto_tmm_enabled"} {
		count := 0
		for _, f := range all {
			if f.name == name {
				count++
			}
		}
		if count != 1 {
			t.Errorf("existing Downloads field %q has %d owners, want exactly one", name, count)
		}
	}
}
