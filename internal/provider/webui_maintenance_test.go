package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

func endpointSchema() rs.Schema {
	var out resource.SchemaResponse
	(&webuiEndpointResource{}).Schema(context.Background(), resource.SchemaRequest{}, &out)
	return out.Schema
}
func authSchema() rs.Schema {
	var out resource.SchemaResponse
	(&webuiAuthResource{}).Schema(context.Background(), resource.SchemaRequest{}, &out)
	return out.Schema
}

func TestWebUIMaintenanceSchemasAndOwnership(t *testing.T) {
	ep := endpointSchema()
	for _, name := range []string{"web_ui_address", "web_ui_port", "use_https", "web_ui_https_cert_path", "web_ui_https_key_path", "post_update_url"} {
		if _, ok := ep.Attributes[name]; !ok {
			t.Errorf("endpoint missing %s", name)
		}
	}
	a := authSchema()
	password, ok := a.Attributes["password_wo"]
	if !ok || !password.IsSensitive() || !password.IsWriteOnly() {
		t.Fatal("WebUI password must be sensitive and write-only")
	}
	for _, key := range []string{"web_ui_address", "web_ui_port", "use_https", "web_ui_https_cert_path", "web_ui_https_key_path", "web_ui_username", "web_ui_password"} {
		for _, f := range allPreferenceFields() {
			if f.wireKey == key {
				t.Errorf("%s has ordinary preference ownership", key)
			}
		}
	}
}

func TestWebUIMaintenanceValidationAndNoOpPatches(t *testing.T) {
	s := endpointSchema()
	cfgState := testState(t, s, map[string]attr.Value{"web_ui_address": types.StringValue("127.0.0.1"), "web_ui_port": types.Int64Value(8080), "post_update_url": types.StringValue("http://127.0.0.1:8080")})
	priorState := testState(t, s, map[string]attr.Value{"id": types.StringValue("application"), "web_ui_address": types.StringValue("127.0.0.1"), "web_ui_port": types.Int64Value(8080), "post_update_url": types.StringValue("http://127.0.0.1:8080")})
	var cfg, prior types.Object
	if d := cfgState.Get(context.Background(), &cfg); d.HasError() {
		t.Fatal(d)
	}
	if d := priorState.Get(context.Background(), &prior); d.HasError() {
		t.Fatal(d)
	}
	var d diag.Diagnostics
	if patch := endpointPatch(cfg, &prior, false, &d); len(patch) != 0 || d.HasError() {
		t.Fatalf("unchanged endpoint must be a no-op: %#v %v", patch, d)
	}
	badState := testState(t, s, map[string]attr.Value{"web_ui_port": types.Int64Value(65536)})
	var bad types.Object
	_ = badState.Get(context.Background(), &bad)
	d = diag.Diagnostics{}
	if endpointPatch(bad, nil, true, &d) != nil || !d.HasError() {
		t.Fatal("out-of-range port accepted")
	}
	bind := webUIBindValidator{}
	var bindResponse validator.StringResponse
	bind.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue("invalid-bind"), Path: path.Root("web_ui_address")}, &bindResponse)
	if !bindResponse.Diagnostics.HasError() {
		t.Fatal("invalid bind address accepted")
	}
}

func TestWebUIAuthValidationAndUnchangedPasswordNotReplayed(t *testing.T) {
	u := webUIUsernameValidator{}
	for _, value := range []string{"ab", "a:b"} {
		var r validator.StringResponse
		u.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(value), Path: path.Root("username")}, &r)
		if !r.Diagnostics.HasError() {
			t.Errorf("username %q accepted", value)
		}
	}
	for _, value := range []string{"a😀", "ééé"} {
		var r validator.StringResponse
		u.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(value), Path: path.Root("username")}, &r)
		if r.Diagnostics.HasError() {
			t.Errorf("native-valid username %q rejected: %v", value, r.Diagnostics)
		}
	}
	p := webUIPasswordValidator{}
	var pr validator.StringResponse
	p.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue("short"), Path: path.Root("password_wo")}, &pr)
	if !pr.Diagnostics.HasError() {
		t.Fatal("short password accepted")
	}
	for _, value := range []string{"😀😀😀", "éééééé"} {
		var r validator.StringResponse
		p.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(value), Path: path.Root("password_wo")}, &r)
		if r.Diagnostics.HasError() {
			t.Errorf("native-valid password rejected: %v", r.Diagnostics)
		}
	}
	s := authSchema()
	priorState := testState(t, s, map[string]attr.Value{"id": types.StringValue("application"), "username": types.StringValue("admin"), "password_wo_version": types.Int64Value(3)})
	cfgState := testState(t, s, map[string]attr.Value{"username": types.StringValue("admin"), "password_wo": types.StringValue("secret-phrase"), "password_wo_version": types.Int64Value(3)})
	planState := testState(t, s, map[string]attr.Value{"id": types.StringValue("application"), "username": types.StringValue("admin"), "password_wo_version": types.Int64Value(3)})
	var prior, cfg, plan types.Object
	_ = priorState.Get(context.Background(), &prior)
	_ = cfgState.Get(context.Background(), &cfg)
	_ = planState.Get(context.Background(), &plan)
	var d diag.Diagnostics
	patch := authPatch(cfg, &prior, plan, false, &d)
	if d.HasError() || len(patch) != 0 {
		t.Fatalf("unchanged credentials must not be replayed: %#v %v", patch, d)
	}
}

func TestEndpointCreatePersistsUncertainPlanAfterLostResponse(t *testing.T) {
	var writes atomic.Int32
	var port atomic.Int32
	port.Store(8080)
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
			writes.Add(1)
			port.Store(18081)
			conn, buf, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = buf.Flush()
				_ = conn.Close()
			}
			return
		}
		if strings.HasSuffix(r.URL.Path, "/app/preferences") {
			_, _ = fmt.Fprintf(w, `{"web_ui_address":"127.0.0.1","web_ui_port":%d,"use_https":false,"web_ui_https_cert_path":"/tmp/cert","web_ui_https_key_path":"/tmp/key","web_ui_username":"admin"}`, port.Load())
			return
		}
		http.NotFound(w, r)
	})
	s := endpointSchema()
	fields := map[string]attr.Value{
		"web_ui_port":     types.Int64Value(18081),
		"post_update_url": types.StringValue("http://127.0.0.1:1"),
	}
	configState := testState(t, s, fields)
	planFields := map[string]attr.Value{"id": types.StringValue("application"), "web_ui_port": types.Int64Value(18081), "post_update_url": types.StringValue("http://127.0.0.1:1")}
	planState := testState(t, s, planFields)
	resp := &resource.CreateResponse{}
	r := &webuiEndpointResource{client: c}
	r.Create(context.Background(), resource.CreateRequest{
		Config: tfsdk.Config{Schema: s, Raw: configState.Raw},
		Plan:   tfsdk.Plan{Schema: s, Raw: planState.Raw},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("uncertain create must remain non-tainted: %v", resp.Diagnostics)
	}
	if resp.Diagnostics.WarningsCount() == 0 {
		t.Fatal("expected sanitized reconnect warning")
	}
	if writes.Load() != 1 {
		t.Fatalf("setPreferences writes = %d, want exactly one", writes.Load())
	}
	var id types.String
	if d := resp.State.GetAttribute(context.Background(), path.Root("id"), &id); d.HasError() || id.ValueString() != "application" {
		t.Fatalf("uncertain resource id not retained: %v %s", d, id.ValueString())
	}
	var observed types.Int64
	if d := resp.State.GetAttribute(context.Background(), path.Root("web_ui_port"), &observed); d.HasError() || observed.ValueInt64() != 18081 {
		t.Fatalf("planned endpoint state not retained after uncertain write: %v %d", d, observed.ValueInt64())
	}
	readResp := &resource.ReadResponse{}
	r.Read(context.Background(), resource.ReadRequest{State: resp.State}, readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("read after recovery: %v", readResp.Diagnostics)
	}
	if d := readResp.State.GetAttribute(context.Background(), path.Root("web_ui_port"), &observed); d.HasError() || observed.ValueInt64() != 18081 {
		t.Fatalf("refresh failed to adopt actual port: %v %d", d, observed.ValueInt64())
	}
	if writes.Load() != 1 {
		t.Fatalf("refresh replayed endpoint write, count=%d", writes.Load())
	}
}

func TestEndpointPartialRejectedWriteRefreshesBeforeExplicitRepair(t *testing.T) {
	address, port := "*", int32(8080)
	var patches []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/preferences") {
			_, _ = fmt.Fprintf(w, `{"web_ui_address":%q,"web_ui_port":%d,"use_https":false,"web_ui_https_cert_path":"/tmp/cert","web_ui_https_key_path":"/tmp/key"}`, address, port)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
			_ = r.ParseForm()
			var patch map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("json")), &patch); err != nil {
				http.Error(w, "bad", 400)
				return
			}
			patches = append(patches, patch)
			if len(patches) == 1 {
				if v, ok := patch["web_ui_address"].(string); ok {
					address = v
				}
				http.Error(w, "partial rejection", http.StatusBadRequest)
				return
			}
			if len(patch) != 1 || patch["web_ui_port"] == nil {
				http.Error(w, "unexpected replay", 400)
				return
			}
			port = int32(patch["web_ui_port"].(float64))
			_, _ = io.WriteString(w, "Ok.")
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	c, err := client.New(client.Config{URL: server.URL, APIKey: "fixture-key"})
	if err != nil {
		t.Fatal(err)
	}
	s := endpointSchema()
	configValues := map[string]attr.Value{"web_ui_address": types.StringValue("127.0.0.1"), "web_ui_port": types.Int64Value(18081), "post_update_url": types.StringValue(server.URL)}
	configState := testState(t, s, configValues)
	planState := testState(t, s, map[string]attr.Value{"id": types.StringValue("application"), "web_ui_address": types.StringValue("127.0.0.1"), "web_ui_port": types.Int64Value(18081), "post_update_url": types.StringValue(server.URL)})
	r := &webuiEndpointResource{client: c}
	created := &resource.CreateResponse{}
	r.Create(context.Background(), resource.CreateRequest{Config: tfsdk.Config{Schema: s, Raw: configState.Raw}, Plan: tfsdk.Plan{Schema: s, Raw: planState.Raw}}, created)
	if created.Diagnostics.HasError() || created.Diagnostics.WarningsCount() == 0 {
		t.Fatalf("partial write should retain state with warning: %v", created.Diagnostics)
	}
	refreshed := &resource.ReadResponse{}
	r.Read(context.Background(), resource.ReadRequest{State: created.State}, refreshed)
	if refreshed.Diagnostics.HasError() {
		t.Fatal(refreshed.Diagnostics)
	}
	var actualAddress types.String
	var actualPort types.Int64
	_ = refreshed.State.GetAttribute(context.Background(), path.Root("web_ui_address"), &actualAddress)
	_ = refreshed.State.GetAttribute(context.Background(), path.Root("web_ui_port"), &actualPort)
	if actualAddress.ValueString() != "127.0.0.1" || actualPort.ValueInt64() != 8080 {
		t.Fatalf("refresh did not read partial server state: %s:%d", actualAddress.ValueString(), actualPort.ValueInt64())
	}
	updated := &resource.UpdateResponse{}
	r.Update(context.Background(), resource.UpdateRequest{Config: tfsdk.Config{Schema: s, Raw: configState.Raw}, Plan: tfsdk.Plan{Schema: s, Raw: planState.Raw}, State: refreshed.State}, updated)
	if updated.Diagnostics.HasError() {
		t.Fatalf("explicit repair update: %v", updated.Diagnostics)
	}
	if len(patches) != 2 || len(patches[0]) != 2 || len(patches[1]) != 1 || patches[1]["web_ui_port"] == nil {
		t.Fatalf("repair replayed already-applied fields: %#v", patches)
	}
	if port != 18081 {
		t.Fatalf("repair port=%d, want 18081", port)
	}
}

func TestAuthCreateUsesSessionCandidateAndKeepsPasswordOutOfState(t *testing.T) {
	username, password := "admin", "oldpass1"
	var writes atomic.Int32
	var failedLogins atomic.Int32
	var banned atomic.Bool // Native max-auth-failure threshold is one.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/v2/auth/login"):
			_ = r.ParseForm()
			if banned.Load() || r.Form.Get("username") != username || r.Form.Get("password") != password {
				failedLogins.Add(1)
				banned.Store(true)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: password, Path: "/"})
			_, _ = io.WriteString(w, "Ok.")
		case strings.HasSuffix(r.URL.Path, "/api/v2/app/setPreferences"):
			if banned.Load() {
				http.Error(w, "IP banned after failed login", http.StatusForbidden)
				return
			}
			_ = r.ParseForm()
			var patch map[string]any
			if json.Unmarshal([]byte(r.Form.Get("json")), &patch) != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if patch["web_ui_username"] != "operator" || patch["web_ui_password"] != "newpass1" {
				http.Error(w, "unexpected patch", http.StatusBadRequest)
				return
			}
			writes.Add(1)
			username, password = "operator", "newpass1"
			_, _ = io.WriteString(w, "Ok.")
		case strings.HasSuffix(r.URL.Path, "/api/v2/app/preferences"):
			cookie, err := r.Cookie("SID")
			if err != nil || cookie.Value != password {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			_, _ = fmt.Fprintf(w, `{"web_ui_username":%q}`, username)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c, err := client.New(client.Config{URL: server.URL, Username: "admin", Password: "oldpass1"})
	if err != nil {
		t.Fatal(err)
	}
	s := authSchema()
	configState := testState(t, s, map[string]attr.Value{"username": types.StringValue("operator"), "password_wo": types.StringValue("newpass1"), "password_wo_version": types.Int64Value(1)})
	planState := testState(t, s, map[string]attr.Value{"id": types.StringValue("application"), "username": types.StringValue("operator"), "password_wo_version": types.Int64Value(1)})
	resp := &resource.CreateResponse{}
	(&webuiAuthResource{client: c}).Create(context.Background(), resource.CreateRequest{Config: tfsdk.Config{Schema: s, Raw: configState.Raw}, Plan: tfsdk.Plan{Schema: s, Raw: planState.Raw}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("session auth create: %v", resp.Diagnostics)
	}
	if writes.Load() != 1 {
		t.Fatalf("auth mutation count = %d, want exactly one", writes.Load())
	}
	if failedLogins.Load() != 0 || banned.Load() {
		t.Fatalf("candidate credentials were tested before rotation: failed logins=%d banned=%v", failedLogins.Load(), banned.Load())
	}
	var stored types.String
	if d := resp.State.GetAttribute(context.Background(), path.Root("password_wo"), &stored); d.HasError() {
		t.Fatal(d)
	}
	if !stored.IsNull() {
		t.Fatalf("write-only password was stored: %#v", stored)
	}
}

func TestAuthCreateRejectsBearerSelfTransitionWithoutMutation(t *testing.T) {
	var writes atomic.Int32
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
			writes.Add(1)
		}
	})
	s := authSchema()
	configState := testState(t, s, map[string]attr.Value{"username": types.StringValue("operator"), "password_wo": types.StringValue("newpass1"), "password_wo_version": types.Int64Value(1)})
	planState := testState(t, s, map[string]attr.Value{"id": types.StringValue("application"), "username": types.StringValue("operator"), "password_wo_version": types.Int64Value(1)})
	resp := &resource.CreateResponse{}
	(&webuiAuthResource{client: c}).Create(context.Background(), resource.CreateRequest{Config: tfsdk.Config{Schema: s, Raw: configState.Raw}, Plan: tfsdk.Plan{Schema: s, Raw: planState.Raw}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("Bearer self-transition must be rejected")
	}
	if writes.Load() != 0 {
		t.Fatalf("unsupported auth mutation sent %d writes", writes.Load())
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("state was persisted even though the mutation was rejected before send")
	}
}

func TestAuthReadMissingUsernameIsStrict(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/preferences") {
			_, _ = io.WriteString(w, `{}`)
			return
		}
		http.NotFound(w, r)
	})
	s := authSchema()
	state := testState(t, s, map[string]attr.Value{"id": types.StringValue("application"), "username": types.StringValue("admin")})
	resp := &resource.ReadResponse{}
	(&webuiAuthResource{client: c}).Read(context.Background(), resource.ReadRequest{State: state}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("missing established username must be an error")
	}
}

func TestEndpointMissingConfiguredReadFieldDoesNotWrite(t *testing.T) {
	var writes atomic.Int32
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/preferences") {
			_, _ = io.WriteString(w, `{}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
			writes.Add(1)
			return
		}
		http.NotFound(w, r)
	})
	s := endpointSchema()
	cfg := testState(t, s, map[string]attr.Value{"web_ui_port": types.Int64Value(18081), "post_update_url": types.StringValue("http://127.0.0.1:18081")})
	plan := testState(t, s, map[string]attr.Value{"id": types.StringValue("application"), "web_ui_port": types.Int64Value(18081), "post_update_url": types.StringValue("http://127.0.0.1:18081")})
	resp := &resource.CreateResponse{}
	(&webuiEndpointResource{client: c}).Create(context.Background(), resource.CreateRequest{Config: tfsdk.Config{Schema: s, Raw: cfg.Raw}, Plan: tfsdk.Plan{Schema: s, Raw: plan.Raw}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("missing configured port read must be an error")
	}
	if writes.Load() != 0 {
		t.Fatalf("missing field triggered %d writes", writes.Load())
	}
}
