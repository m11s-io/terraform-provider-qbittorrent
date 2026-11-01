package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCredentialsSchemaIsWriteOnlyAndSingleton(t *testing.T) {
	var out resource.SchemaResponse
	(&credentialsResource{}).Schema(context.Background(), resource.SchemaRequest{}, &out)
	for _, name := range []string{"smtp_password_wo", "proxy_password_wo", "dyndns_password_wo"} {
		attr, ok := out.Schema.Attributes[name]
		if !ok || !attr.IsSensitive() || !attr.IsWriteOnly() {
			t.Errorf("%s must be write-only and sensitive", name)
		}
	}
	for _, name := range []string{"smtp_password_wo_version", "proxy_password_wo_version", "dyndns_password_wo_version"} {
		if _, ok := out.Schema.Attributes[name]; !ok {
			t.Errorf("missing version counter %s", name)
		}
	}
}

func credentialsSchema() rschema.Schema {
	var out resource.SchemaResponse
	(&credentialsResource{}).Schema(context.Background(), resource.SchemaRequest{}, &out)
	return out.Schema
}

func credentialsConfig(t *testing.T, s rschema.Schema, configured map[string]any) tfsdk.Config {
	t.Helper()
	values := map[string]tftypes.Value{}
	for name, typ := range s.Type().(types.ObjectType).AttrTypes {
		var value tftypes.Value
		switch typ {
		case types.StringType:
			value = tftypes.NewValue(tftypes.String, nil)
		case types.Int64Type:
			value = tftypes.NewValue(tftypes.Number, nil)
		}
		if configuredValue, ok := configured[name]; ok {
			switch v := configuredValue.(type) {
			case string:
				value = tftypes.NewValue(tftypes.String, v)
			case int64:
				value = tftypes.NewValue(tftypes.Number, v)
			}
		}
		values[name] = value
	}
	return tfsdk.Config{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), values)}
}

func credentialsPlan(t *testing.T, s rschema.Schema, versions ...int64) tfsdk.Plan {
	t.Helper()
	attrs := map[string]attr.Value{"id": types.StringValue("application")}
	for i, name := range []string{"smtp_password_wo_version", "proxy_password_wo_version", "dyndns_password_wo_version"} {
		if i < len(versions) {
			attrs[name] = types.Int64Value(versions[i])
		}
	}
	plan := testState(t, s, attrs)
	return tfsdk.Plan{Schema: s, Raw: plan.Raw}
}

func TestCredentialsCreateRotateAndNoReplay(t *testing.T) {
	var writes atomic.Int32
	var last map[string]any
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
			_ = r.ParseForm()
			last = map[string]any{}
			_ = json.Unmarshal([]byte(r.Form.Get("json")), &last)
			writes.Add(1)
		}
		if strings.HasSuffix(r.URL.Path, "/app/preferences") {
			_, _ = w.Write([]byte(`{"mail_notification_password":"fixture-smtp","proxy_password":"fixture-proxy","dyndns_password":"fixture-dyndns","web_ui_api_key":"fixture-key"}`))
		}
	})
	r := &credentialsResource{client: c}
	s := credentialsSchema()
	config := credentialsConfig(t, s, map[string]any{
		"smtp_password_wo": "smtp-canary-01", "smtp_password_wo_version": int64(1),
		"proxy_password_wo": "proxy-canary-01", "proxy_password_wo_version": int64(2),
		"dyndns_password_wo": "dyndns-canary-01", "dyndns_password_wo_version": int64(3),
	})
	var create resource.CreateResponse
	create.State = tfsdk.State{Schema: s}
	r.Create(context.Background(), resource.CreateRequest{Config: config, Plan: credentialsPlan(t, s, 1, 2, 3)}, &create)
	if create.Diagnostics.HasError() {
		t.Fatal(create.Diagnostics)
	}
	if writes.Load() != 1 || len(last) != 3 || last["mail_notification_password"] != "smtp-canary-01" || last["proxy_password"] != "proxy-canary-01" || last["dyndns_password"] != "dyndns-canary-01" {
		t.Fatalf("create credential patch = %#v, writes=%d", last, writes.Load())
	}
	if strings.Contains(create.State.Raw.String(), "canary") {
		t.Fatal("password canary persisted in resource state")
	}
	if strings.Contains(fmt.Sprint(create.Diagnostics.Errors()), "canary") {
		t.Fatal("password canary leaked through diagnostics")
	}

	prior := create.State
	rotateConfig := credentialsConfig(t, s, map[string]any{
		"smtp_password_wo": "smtp-canary-02", "smtp_password_wo_version": int64(4),
		"proxy_password_wo": "proxy-canary-01", "proxy_password_wo_version": int64(2),
		"dyndns_password_wo": "dyndns-canary-01", "dyndns_password_wo_version": int64(3),
	})
	var update resource.UpdateResponse
	update.State = tfsdk.State{Schema: s}
	r.Update(context.Background(), resource.UpdateRequest{Config: rotateConfig, Plan: credentialsPlan(t, s, 4, 2, 3), State: prior}, &update)
	if update.Diagnostics.HasError() {
		t.Fatal(update.Diagnostics)
	}
	if writes.Load() != 2 || len(last) != 1 || last["mail_notification_password"] != "smtp-canary-02" {
		t.Fatalf("rotation must send only changed secret: %#v", last)
	}
	if strings.Contains(update.State.Raw.String(), "canary") {
		t.Fatal("rotated password canary persisted in resource state")
	}

	prior = update.State
	unchangedConfig := credentialsConfig(t, s, map[string]any{
		"smtp_password_wo": "smtp-canary-02", "smtp_password_wo_version": int64(4),
		"proxy_password_wo": "proxy-canary-01", "proxy_password_wo_version": int64(2),
		"dyndns_password_wo": "dyndns-canary-01", "dyndns_password_wo_version": int64(3),
	})
	var unchanged resource.UpdateResponse
	unchanged.State = tfsdk.State{Schema: s}
	r.Update(context.Background(), resource.UpdateRequest{Config: unchangedConfig, Plan: credentialsPlan(t, s, 4, 2, 3), State: prior}, &unchanged)
	if unchanged.Diagnostics.HasError() {
		t.Fatal(unchanged.Diagnostics)
	}
	if writes.Load() != 2 {
		t.Fatalf("unchanged versions replayed secrets: writes=%d", writes.Load())
	}
	if strings.Contains(unchanged.State.Raw.String(), "canary") {
		t.Fatal("unchanged apply persisted a password canary")
	}
}

func TestCredentialsNullRelinquishesAndEmptyInputClears(t *testing.T) {
	var writes atomic.Int32
	var last map[string]any
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
			_ = r.ParseForm()
			last = map[string]any{}
			_ = json.Unmarshal([]byte(r.Form.Get("json")), &last)
			writes.Add(1)
		}
	})
	r := &credentialsResource{client: c}
	s := credentialsSchema()
	prior := credentialsPlan(t, s, 1, 1, 1)
	state := tfsdk.State{Schema: s, Raw: prior.Raw}
	clearConfig := credentialsConfig(t, s, map[string]any{
		"smtp_password_wo": "", "smtp_password_wo_version": int64(2),
	})
	var clear resource.UpdateResponse
	clear.State = tfsdk.State{Schema: s}
	r.Update(context.Background(), resource.UpdateRequest{Config: clearConfig, Plan: credentialsPlan(t, s, 2), State: state}, &clear)
	if clear.Diagnostics.HasError() || writes.Load() != 1 || len(last) != 1 || last["mail_notification_password"] != "" {
		t.Fatalf("explicit empty password did not clear: patch=%#v diagnostics=%v", last, clear.Diagnostics)
	}
	state = clear.State
	relinquishConfig := credentialsConfig(t, s, nil)
	var relinquish resource.UpdateResponse
	relinquish.State = tfsdk.State{Schema: s}
	r.Update(context.Background(), resource.UpdateRequest{Config: relinquishConfig, Plan: credentialsPlan(t, s), State: state}, &relinquish)
	if relinquish.Diagnostics.HasError() || writes.Load() != 1 {
		t.Fatalf("null relinquishment mutated credentials: writes=%d diagnostics=%v", writes.Load(), relinquish.Diagnostics)
	}
}

func TestCredentialsRejectsVersionChangesWithoutMatchingSecret(t *testing.T) {
	s := credentialsSchema()
	config := credentialsConfig(t, s, map[string]any{"smtp_password_wo_version": int64(2)})
	var cfg, plan types.Object
	if diags := config.Get(context.Background(), &cfg); diags.HasError() {
		t.Fatal(diags)
	}
	p := credentialsPlan(t, s, 2)
	if diags := p.Get(context.Background(), &plan); diags.HasError() {
		t.Fatal(diags)
	}
	d := diag.Diagnostics{}
	patch := buildVersionedSecretPatch(cfg, plan, types.ObjectNull(plan.AttributeTypes(context.Background())), true, &d)
	if !d.HasError() || len(patch) != 0 {
		t.Fatalf("missing write-only value was accepted: patch=%v diagnostics=%v", patch, d)
	}

	priorState := credentialsPlan(t, s, 3)
	var prior types.Object
	if diags := priorState.Get(context.Background(), &prior); diags.HasError() {
		t.Fatal(diags)
	}
	rotate := credentialsConfig(t, s, map[string]any{"smtp_password_wo": "value", "smtp_password_wo_version": int64(2)})
	if diags := rotate.Get(context.Background(), &cfg); diags.HasError() {
		t.Fatal(diags)
	}
	d = diag.Diagnostics{}
	patch = buildVersionedSecretPatch(cfg, plan, prior, false, &d)
	if !d.HasError() || len(patch) != 0 {
		t.Fatalf("decreasing version was accepted: patch=%v diagnostics=%v", patch, d)
	}
}

func TestCredentialsDestroyDoesNotMutate(t *testing.T) {
	var writes atomic.Int32
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
			writes.Add(1)
		}
	})
	r := &credentialsResource{client: c}
	var out resource.DeleteResponse
	r.Delete(context.Background(), resource.DeleteRequest{State: tfsdk.State{Schema: credentialsSchema()}}, &out)
	if out.Diagnostics.HasError() || writes.Load() != 0 {
		t.Fatalf("destroy mutated credentials or failed: writes=%d diag=%v", writes.Load(), out.Diagnostics)
	}
}

func TestCredentialsImportKeepsOnlyApplicationID(t *testing.T) {
	var response resource.ImportStateResponse
	response.State = testState(t, credentialsSchema(), nil)
	(&credentialsResource{}).ImportState(context.Background(), resource.ImportStateRequest{ID: "application"}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	if !strings.Contains(response.State.Raw.String(), `"application"`) {
		t.Fatalf("import did not set application ID: %s", response.State.Raw.String())
	}
}
