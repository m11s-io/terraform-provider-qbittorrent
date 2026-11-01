package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWebUISettingsMetadataAndSensitiveHeaders(t *testing.T) {
	want := []string{
		"web_ui_upnp",
		"bypass_local_auth", "bypass_auth_subnet_whitelist_enabled", "bypass_auth_subnet_whitelist",
		"web_ui_max_auth_fail_count", "web_ui_ban_duration", "web_ui_session_timeout", "web_ui_sessions_count_limit",
		"alternative_webui_enabled", "alternative_webui_path", "web_ui_clickjacking_protection_enabled",
		"web_ui_csrf_protection_enabled", "web_ui_secure_cookie_enabled", "web_ui_host_header_validation_enabled",
		"web_ui_domain_list", "web_ui_use_custom_http_headers_enabled", "web_ui_custom_http_headers",
		"web_ui_reverse_proxy_enabled", "web_ui_reverse_proxies_list", "dyndns_enabled", "dyndns_service",
		"dyndns_domain", "dyndns_username",
	}
	if len(webUISettingsPreferenceFields) != len(want) {
		t.Fatalf("got %d WebUI settings, want %d", len(webUISettingsPreferenceFields), len(want))
	}
	for _, name := range want {
		f, ok := findPreferenceField(webUISettingsPreferenceFields, name)
		if !ok || f.tab != "WebUI" || f.wireKey != name {
			t.Errorf("missing or invalid metadata for %q: %+v", name, f)
		}
	}
	for _, name := range []string{"web_ui_address", "web_ui_port", "use_https", "web_ui_https_cert_path", "web_ui_https_key_path", "web_ui_username", "web_ui_password"} {
		if _, ok := findPreferenceField(webUISettingsPreferenceFields, name); ok {
			t.Errorf("Task 5 must not own listener/auth field %q", name)
		}
	}
	headers, ok := findPreferenceField(webUISettingsPreferenceFields, "web_ui_custom_http_headers")
	if !ok || !headers.sensitive || !headers.excludeFromDataSource {
		t.Fatalf("custom headers must be sensitive and excluded from the generic data source: %+v", headers)
	}
	service, ok := findPreferenceField(webUISettingsPreferenceFields, "dyndns_service")
	if !ok || len(service.integerEnums) != 2 || service.integerEnums[0] != 0 || service.integerEnums[1] != 1 {
		t.Fatalf("DynDNS service enum = %+v", service)
	}
}

func TestWebUISettingsNativeIntegerBoundsAndEnums(t *testing.T) {
	for _, tc := range []struct {
		name    string
		minimum int64
		maximum int64
	}{
		{"web_ui_max_auth_fail_count", -1 << 31, 1<<31 - 1},
		{"web_ui_ban_duration", -1 << 31, 1<<31 - 1},
		{"web_ui_session_timeout", -1 << 31, 1<<31 - 1},
		{"web_ui_sessions_count_limit", -1 << 31, 1<<31 - 1},
	} {
		f, ok := findPreferenceField(webUISettingsPreferenceFields, tc.name)
		if !ok {
			t.Fatalf("missing %s", tc.name)
		}
		if f.integerMin == nil || *f.integerMin != tc.minimum || f.integerMax == nil || *f.integerMax != tc.maximum {
			t.Errorf("%s must accept native signed int32 range, metadata=%+v", tc.name, f)
		}
		for _, value := range []struct {
			v     int64
			valid bool
		}{{tc.minimum, true}, {-1, true}, {0, true}, {tc.maximum, true}, {tc.minimum - 1, false}, {tc.maximum + 1, false}} {
			var resp validator.Int64Response
			(preferenceIntValidator{field: f}).ValidateInt64(context.Background(), validator.Int64Request{Path: path.Root(tc.name), ConfigValue: types.Int64Value(value.v)}, &resp)
			if got := !resp.Diagnostics.HasError(); got != value.valid {
				t.Errorf("%s=%d valid=%v, want %v", tc.name, value.v, got, value.valid)
			}
		}
	}
	var schema resource.SchemaResponse
	(&preferencesResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schema)
	if _, ok := schema.Schema.Attributes["web_ui_custom_http_headers"]; !ok {
		t.Fatal("custom headers missing from resource schema")
	}
}

func TestWebUISettingsPatchPreservesProtectionDefaultsAndSecrets(t *testing.T) {
	remote := map[string]any{
		"web_ui_clickjacking_protection_enabled": true,
		"web_ui_csrf_protection_enabled":         true,
		"web_ui_host_header_validation_enabled":  true,
		"web_ui_secure_cookie_enabled":           true,
		"mail_notification_password":             "smtp-canary",
		"proxy_password":                         "proxy-canary",
		"dyndns_password":                        "dyndns-canary",
		"web_ui_api_key":                         "api-key-canary",
	}
	var writes atomic.Int32
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/app/setPreferences") {
			_ = r.ParseForm()
			var patch map[string]any
			_ = json.Unmarshal([]byte(r.Form.Get("json")), &patch)
			if len(patch) != 2 || patch["web_ui_max_auth_fail_count"] != float64(7) || patch["web_ui_upnp"] != false {
				t.Errorf("unexpected sparse WebUI update: %#v", patch)
			}
			for key, value := range patch {
				remote[key] = value
			}
			writes.Add(1)
			return
		}
		_ = json.NewEncoder(w).Encode(remote)
	})
	plan := testState(t, prefsSchema(), map[string]attr.Value{"web_ui_max_auth_fail_count": types.Int64Value(7), "web_ui_upnp": types.BoolValue(false)})
	var out resource.CreateResponse
	out.State = tfsdk.State{Schema: plan.Schema}
	(&preferencesResource{client: c}).Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: plan.Schema, Raw: plan.Raw}}, &out)
	if out.Diagnostics.HasError() {
		t.Fatal(out.Diagnostics)
	}
	if writes.Load() != 1 {
		t.Fatalf("got %d preference writes, want one", writes.Load())
	}
	for _, key := range []string{"web_ui_clickjacking_protection_enabled", "web_ui_csrf_protection_enabled", "web_ui_host_header_validation_enabled", "web_ui_secure_cookie_enabled"} {
		if remote[key] != true {
			t.Errorf("security protection %s changed: %#v", key, remote[key])
		}
	}
	if strings.Contains(out.State.Raw.String(), "canary") {
		t.Fatal("a password or API key canary reached resource state")
	}
}
