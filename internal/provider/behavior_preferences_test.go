package provider

import (
	"encoding/json"
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

func TestWebUIResourceOwnership(t *testing.T) {
	remote := map[string]any{"qbt_color_scheme": "dark", "unmanaged": "untouched"}
	writes := 0
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if strings.HasSuffix(r.URL.Path, "/store") {
			writes++
			var patch map[string]any
			json.Unmarshal([]byte(r.Form.Get("data")), &patch)
			if len(patch) != 2 || patch["qbt_color_scheme"] != nil || patch["qbt_use_virtual_list"] != false {
				t.Fatalf("wrong partial patch: %v", patch)
			}
			for k, v := range patch {
				if v == nil {
					delete(remote, k)
				} else {
					remote[k] = v
				}
			}
		} else {
			json.NewEncoder(w).Encode(remote)
		}
	})
	r := &preferencesResource{client: c, webui: true}
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	s := testState(t, sr.Schema, map[string]attr.Value{"color_scheme": types.StringValue("auto"), "use_virtual_list": types.BoolValue(false)})
	out := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if out.Diagnostics.HasError() {
		t.Fatal(out.Diagnostics)
	}
	if writes != 1 || remote["unmanaged"] != "untouched" {
		t.Fatal("unmanaged settings changed")
	}
	remote["qbt_color_scheme"] = "light"
	rd := resource.ReadResponse{State: out.State}
	r.Read(ctx, resource.ReadRequest{State: out.State}, &rd)
	if rd.Diagnostics.HasError() || attrString(t, rd.State, "color_scheme").ValueString() != "light" {
		t.Fatal("drift not reflected", rd.Diagnostics)
	}
	relinquish := testState(t, sr.Schema, map[string]attr.Value{"id": types.StringValue("application")})
	update := resource.UpdateResponse{State: relinquish}
	r.Update(ctx, resource.UpdateRequest{Plan: tfsdk.Plan{Schema: relinquish.Schema, Raw: relinquish.Raw}}, &update)
	if update.Diagnostics.HasError() || writes != 1 {
		t.Fatal("relinquishing ownership changed settings", update.Diagnostics)
	}
	var del resource.DeleteResponse
	r.Delete(ctx, resource.DeleteRequest{State: out.State}, &del)
	if writes != 1 {
		t.Fatal("destroy reset settings")
	}
}

func TestBehaviorValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		min, max int64
	}{
		{"file_log_max_size", 1, 1024000}, {"file_log_age", 1, 365}, {"file_log_age_type", 0, 2},
	} {
		a := prefsSchema().Attributes[tc.name].(interface{ Int64Validators() []validator.Int64 })
		for _, n := range []int64{tc.min - 1, tc.min, tc.max, tc.max + 1} {
			var out validator.Int64Response
			for _, v := range a.Int64Validators() {
				v.ValidateInt64(ctx, validator.Int64Request{Path: path.Root(tc.name), ConfigValue: types.Int64Value(n)}, &out)
			}
			if out.Diagnostics.HasError() != (n < tc.min || n > tc.max) {
				t.Errorf("%s=%d: %v", tc.name, n, out.Diagnostics)
			}
		}
	}
	for _, tc := range []struct{ name, valid, invalid string }{
		{"date_format", "default", "yyyy"}, {"color_scheme", "auto", "blue"},
		{"display_density", "compact", "tiny"}, {"dblclick_download", "0", "2"},
		{"dblclick_complete", "1", "-1"}, {"dblclick_filter", "1", "reset"},
	} {
		for _, value := range []string{tc.valid, tc.invalid} {
			var out validator.StringResponse
			for _, v := range webUIStringValidators(tc.name, true) {
				v.ValidateString(ctx, validator.StringRequest{Path: path.Root(tc.name), ConfigValue: types.StringValue(value)}, &out)
			}
			if out.Diagnostics.HasError() != (value == tc.invalid) {
				t.Errorf("%s=%q: %v", tc.name, value, out.Diagnostics)
			}
		}
	}
}

func TestBehaviorPartialMapping(t *testing.T) {
	want := map[string]any{"locale": "en", "confirm_torrent_deletion": false, "status_bar_external_ip": true, "store_search_jobs": false, "store_search_job_results": false, "performance_warning": false, "file_log_enabled": false, "file_log_path": "/logs", "file_log_backup_enabled": false, "file_log_max_size": float64(65), "file_log_delete_old": false, "file_log_age": float64(1), "file_log_age_type": float64(0)}
	fields := map[string]attr.Value{}
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
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "setPreferences") {
			r.ParseForm()
			var got map[string]any
			json.Unmarshal([]byte(r.Form.Get("json")), &got)
			if len(got) != len(want) {
				t.Fatalf("wrong patch: %v", got)
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("wrong %s: %v", k, got[k])
				}
			}
		} else {
			json.NewEncoder(w).Encode(want)
		}
	})
	r := &preferencesResource{client: c}
	s := testState(t, prefsSchema(), fields)
	var out resource.CreateResponse
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if out.Diagnostics.HasError() {
		t.Fatal(out.Diagnostics)
	}
	var age types.Int64
	out.State.GetAttribute(ctx, path.Root("file_log_age_type"), &age)
	if age.ValueInt64() != 0 || age.IsNull() {
		t.Fatal("explicit zero was dropped")
	}
}
