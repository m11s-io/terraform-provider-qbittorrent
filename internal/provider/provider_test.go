package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

var ctx = context.Background()

func testState(t *testing.T, s rschema.Schema, fields map[string]attr.Value) tfsdk.State {
	t.Helper()
	ats := s.Type().(types.ObjectType).AttrTypes
	vals := map[string]attr.Value{}
	for k, typ := range ats {
		switch typ {
		case types.StringType:
			vals[k] = types.StringNull()
		case types.BoolType:
			vals[k] = types.BoolNull()
		case types.Int64Type:
			vals[k] = types.Int64Null()
		}
	}
	for k, v := range fields {
		vals[k] = v
	}
	state := tfsdk.State{Schema: s}
	d := state.Set(ctx, types.ObjectValueMust(ats, vals))
	if d.HasError() {
		t.Fatal(d)
	}
	return state
}
func prefsSchema() rschema.Schema {
	var out resource.SchemaResponse
	(&preferencesResource{}).Schema(ctx, resource.SchemaRequest{}, &out)
	return out.Schema
}
func categorySchema() rschema.Schema {
	var out resource.SchemaResponse
	(&categoryResource{}).Schema(ctx, resource.SchemaRequest{}, &out)
	return out.Schema
}
func fakeClient(t *testing.T, h http.HandlerFunc) *client.Client {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	c, e := client.New(client.Config{URL: s.URL, APIKey: "test"})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func attrString(t *testing.T, s tfsdk.State, key string) types.String {
	t.Helper()
	var v types.String
	if d := s.GetAttribute(ctx, path.Root(key), &v); d.HasError() {
		t.Fatal(d)
	}
	return v
}
func TestPreferencesOwnershipLifecycle(t *testing.T) {
	var writes atomic.Int32
	remote := map[string]any{"dht": true, "dl_limit": float64(1024), "save_path": "/unmanaged", "web_ui_api_key": "secret"}
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "setPreferences") {
			writes.Add(1)
			r.ParseForm()
			var patch map[string]any
			json.Unmarshal([]byte(r.Form.Get("json")), &patch)
			if len(patch) != 2 || patch["dht"] != false || patch["dl_limit"] != float64(0) {
				t.Errorf("bad ownership patch: %v", patch)
			}
			for k, v := range patch {
				remote[k] = v
			}
			return
		}
		json.NewEncoder(w).Encode(remote)
	})
	r := &preferencesResource{client: c}
	s := testState(t, prefsSchema(), map[string]attr.Value{"dht": types.BoolValue(false), "dl_limit": types.Int64Value(0)})
	var create resource.CreateResponse
	create.State = tfsdk.State{Schema: s.Schema}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &create)
	if create.Diagnostics.HasError() {
		t.Fatal(create.Diagnostics)
	}
	if attrString(t, create.State, "id").ValueString() != "application" || !attrString(t, create.State, "save_path").IsNull() {
		t.Fatal("claimed unmanaged setting")
	}
	remote["dht"] = true
	read := resource.ReadResponse{State: create.State}
	r.Read(ctx, resource.ReadRequest{State: create.State}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	var dht types.Bool
	read.State.GetAttribute(ctx, path.Root("dht"), &dht)
	if !dht.ValueBool() {
		t.Fatal("drift not detected")
	}
	// Null configuration relinquishes ownership without resetting remote preferences.
	relinquish := testState(t, prefsSchema(), map[string]attr.Value{"id": types.StringValue("application")})
	update := resource.UpdateResponse{State: relinquish}
	r.Update(ctx, resource.UpdateRequest{Plan: tfsdk.Plan{Schema: relinquish.Schema, Raw: relinquish.Raw}}, &update)
	if update.Diagnostics.HasError() {
		t.Fatal(update.Diagnostics)
	}
	var del resource.DeleteResponse
	r.Delete(ctx, resource.DeleteRequest{State: create.State}, &del)
	if del.Diagnostics.HasError() || writes.Load() != 1 || remote["save_path"] != "/unmanaged" {
		t.Fatal("destroy/reset changed preferences")
	}
	imp := resource.ImportStateResponse{State: testState(t, prefsSchema(), nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "application"}, &imp)
	if imp.Diagnostics.HasError() {
		t.Fatal(imp.Diagnostics)
	}
	read = resource.ReadResponse{State: imp.State}
	r.Read(ctx, resource.ReadRequest{State: imp.State}, &read)
	if read.Diagnostics.HasError() || !attrString(t, read.State, "save_path").IsNull() {
		t.Fatal("import claimed settings")
	}
	bad := resource.ImportStateResponse{State: imp.State}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "wrong"}, &bad)
	if !bad.Diagnostics.HasError() {
		t.Fatal("bad import accepted")
	}
}
func TestPreferencesRejectedNormalization(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "preferences") {
			fmt.Fprint(w, `{"save_path":"/normalized"}`)
		}
	})
	s := testState(t, prefsSchema(), map[string]attr.Value{"save_path": types.StringValue("relative")})
	r := &preferencesResource{client: c}
	out := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if !out.Diagnostics.HasError() {
		t.Fatal("normalization silently accepted")
	}
}
func TestPreferencesDataSourceExcludesCredentials(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"locale":"en","confirm_torrent_deletion":true,"status_bar_external_ip":false,"store_search_jobs":false,"store_search_job_results":false,"performance_warning":false,"file_log_enabled":false,"file_log_path":"/logs","file_log_backup_enabled":false,"file_log_max_size":65,"file_log_delete_old":false,"file_log_age":1,"file_log_age_type":1,"save_path":"/data","temp_path":"/tmp","temp_path_enabled":false,"auto_tmm_enabled":false,"dl_limit":0,"up_limit":0,"alt_dl_limit":1024,"alt_up_limit":1024,"scheduler_enabled":false,"schedule_from_hour":8,"schedule_from_min":0,"schedule_to_hour":20,"schedule_to_min":0,"scheduler_days":0,"limit_utp_rate":true,"limit_tcp_overhead":false,"limit_lan_peers":true,"dht":true,"pex":true,"lsd":true,"listen_port":6881,"upnp":false,"web_ui_api_key":"secret"}`)
	})
	d := &preferencesDataSource{client: c}
	var schema datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schema)
	var _ dschema.Schema = schema.Schema
	out := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	d.Read(ctx, datasource.ReadRequest{}, &out)
	if out.Diagnostics.HasError() {
		t.Fatal(out.Diagnostics)
	}
	if strings.Contains(out.State.Raw.String(), "secret") {
		t.Fatal("secret in data source")
	}
}
func TestCategoryLifecycleAndSafety(t *testing.T) {
	cats := map[string]client.Category{}
	var deletes atomic.Int32
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		name := r.Form.Get("category")
		switch {
		case strings.HasSuffix(r.URL.Path, "/categories"):
			json.NewEncoder(w).Encode(cats)
		case strings.HasSuffix(r.URL.Path, "createCategory"):
			if _, ok := cats[name]; ok {
				w.WriteHeader(409)
				return
			}
			cats[name] = client.Category{Name: name, SavePath: r.Form.Get("savePath")}
		case strings.HasSuffix(r.URL.Path, "editCategory"):
			cats[name] = client.Category{Name: name, SavePath: r.Form.Get("savePath")}
		case strings.HasSuffix(r.URL.Path, "removeCategories"):
			deletes.Add(1)
			delete(cats, r.Form.Get("categories"))
		default:
			t.Errorf("unsafe endpoint: %s", r.URL.Path)
		}
	})
	r := &categoryResource{client: c}
	s := testState(t, categorySchema(), map[string]attr.Value{"name": types.StringValue("datasets"), "save_path": types.StringValue("/data")})
	create := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &create)
	if create.Diagnostics.HasError() {
		t.Fatal(create.Diagnostics)
	}
	dup := resource.CreateResponse{State: tfsdk.State{Schema: s.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &dup)
	if !dup.Diagnostics.HasError() || !strings.Contains(fmt.Sprint(dup.Diagnostics), "import") {
		t.Fatal("duplicate adopted")
	}
	changed := testState(t, s.Schema.(rschema.Schema), map[string]attr.Value{"id": types.StringValue("datasets"), "name": types.StringValue("datasets"), "save_path": types.StringValue("/changed")})
	update := resource.UpdateResponse{State: create.State}
	r.Update(ctx, resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: changed.Raw}}, &update)
	if update.Diagnostics.HasError() || cats["datasets"].SavePath != "/changed" {
		t.Fatal(update.Diagnostics)
	}
	cats["datasets/samples"] = client.Category{Name: "datasets/samples"}
	var blocked resource.DeleteResponse
	r.Delete(ctx, resource.DeleteRequest{State: update.State}, &blocked)
	if !blocked.Diagnostics.HasError() || deletes.Load() != 0 {
		t.Fatal("deleted descendant")
	}
	delete(cats, "datasets/samples")
	var del resource.DeleteResponse
	r.Delete(ctx, resource.DeleteRequest{State: update.State}, &del)
	if del.Diagnostics.HasError() || deletes.Load() != 1 {
		t.Fatal(del.Diagnostics)
	}
	read := resource.ReadResponse{State: update.State}
	r.Read(ctx, resource.ReadRequest{State: update.State}, &read)
	if read.Diagnostics.HasError() || !read.State.Raw.IsNull() {
		t.Fatal("missing category retained")
	}
	cats["datasets"] = client.Category{Name: "datasets", SavePath: "/imported"}
	imp := resource.ImportStateResponse{State: testState(t, s.Schema.(rschema.Schema), nil)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "datasets"}, &imp)
	read = resource.ReadResponse{State: imp.State}
	r.Read(ctx, resource.ReadRequest{State: imp.State}, &read)
	if read.Diagnostics.HasError() || attrString(t, read.State, "save_path").ValueString() != "/imported" {
		t.Fatal("import failed")
	}
}
func TestProviderEnvironmentPrecedence(t *testing.T) {
	var auth atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer explicit" {
			auth.Store(true)
		}
		if strings.HasSuffix(r.URL.Path, "webapiVersion") {
			fmt.Fprint(w, "2.16.2")
		} else {
			fmt.Fprint(w, "v5.3.0")
		}
	}))
	defer server.Close()
	t.Setenv("QBITTORRENT_URL", server.URL)
	t.Setenv("QBITTORRENT_API_KEY", "env")
	p := New("test")()
	var sr provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &sr)
	var _ pschema.Schema = sr.Schema
	ats := sr.Schema.Type().(types.ObjectType).AttrTypes
	vals := map[string]attr.Value{"url": types.StringNull(), "api_key": types.StringValue("explicit"), "username": types.StringNull(), "password": types.StringNull()}
	state := tfsdk.State{Schema: sr.Schema}
	state.Set(ctx, types.ObjectValueMust(ats, vals))
	out := provider.ConfigureResponse{}
	p.Configure(ctx, provider.ConfigureRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: state.Raw}}, &out)
	if out.Diagnostics.HasError() || !auth.Load() {
		t.Fatalf("precedence failed: %v", out.Diagnostics)
	}
	vals["username"] = types.StringValue("mixed")
	state.Set(ctx, types.ObjectValueMust(ats, vals))
	out = provider.ConfigureResponse{}
	p.Configure(ctx, provider.ConfigureRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: state.Raw}}, &out)
	if !out.Diagnostics.HasError() {
		t.Fatal("mixed authentication accepted")
	}
}

func TestNumericAndCategoryValidation(t *testing.T) {
	for _, tt := range []struct {
		port bool
		n    int64
		bad  bool
	}{{false, 0, false}, {false, 1024, false}, {false, 2147482624, false}, {false, 1, true}, {false, -1, true}, {false, 2147483648, true}, {true, 0, true}, {true, 1, false}, {true, 65535, false}, {true, 65536, true}} {
		var out validator.Int64Response
		numberValidator{port: tt.port}.ValidateInt64(ctx, validator.Int64Request{Path: path.Root("value"), ConfigValue: types.Int64Value(tt.n)}, &out)
		if out.Diagnostics.HasError() != tt.bad {
			t.Errorf("port=%v number=%d: %v", tt.port, tt.n, out.Diagnostics)
		}
	}
	for _, name := range []string{"", "/root", "root/", "a//b", "a\\b", "a\nb", "a\rb"} {
		if validCategoryName(name) {
			t.Errorf("unsafe name accepted %q", name)
		}
	}
	for _, name := range []string{"a", "a/b", "Research & Data"} {
		if !validCategoryName(name) {
			t.Errorf("valid name rejected %q", name)
		}
	}
}

func TestRejectedPreferenceWritePreservesState(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	r := &preferencesResource{client: c}
	planned := testState(t, prefsSchema(), map[string]attr.Value{"id": types.StringUnknown(), "dl_limit": types.Int64Value(2048)})
	empty := tfsdk.State{Schema: planned.Schema, Raw: tftypes.NewValue(planned.Schema.Type().TerraformType(ctx), nil)}
	created := resource.CreateResponse{State: empty}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: planned.Schema, Raw: planned.Raw}}, &created)
	if !created.Diagnostics.HasError() || !created.State.Raw.IsNull() {
		t.Error("rejected Create returned planned state", created.Diagnostics, created.State.Raw)
	}
	prior := testState(t, prefsSchema(), map[string]attr.Value{"id": types.StringValue("application"), "dl_limit": types.Int64Value(1024)})
	planned.SetAttribute(ctx, path.Root("id"), "application")
	updated := resource.UpdateResponse{State: prior}
	r.Update(ctx, resource.UpdateRequest{State: prior, Plan: tfsdk.Plan{Schema: planned.Schema, Raw: planned.Raw}}, &updated)
	var value types.Int64
	updated.State.GetAttribute(ctx, path.Root("dl_limit"), &value)
	if !updated.Diagnostics.HasError() || value.ValueInt64() != 1024 {
		t.Fatal("rejected Update replaced prior state", updated.Diagnostics, value)
	}
}
func TestNestedCategoryRequiresExistingAncestors(t *testing.T) {
	var writes atomic.Int32
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			writes.Add(1)
			return
		}
		fmt.Fprint(w, `{}`)
	})
	r := &categoryResource{client: c}
	planned := testState(t, categorySchema(), map[string]attr.Value{"id": types.StringUnknown(), "name": types.StringValue("parent/child"), "save_path": types.StringValue("")})
	out := resource.CreateResponse{State: tfsdk.State{Schema: planned.Schema, Raw: tftypes.NewValue(planned.Schema.Type().TerraformType(ctx), nil)}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: planned.Schema, Raw: planned.Raw}}, &out)
	if !out.Diagnostics.HasError() || writes.Load() != 0 || !out.State.Raw.IsNull() {
		t.Fatal("silently created ancestor", out.Diagnostics, writes.Load())
	}
}
