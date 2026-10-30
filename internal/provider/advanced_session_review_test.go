package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRestoreAdvancedBackendRecoversAndRestoresIndependently(t *testing.T) {
	backend := "SQLite"
	serverAvailable := false // Models a failed Terraform destroy leaving WebUI unavailable.
	var events []string
	read := func(context.Context) (string, error) {
		events = append(events, "read")
		if !serverAvailable {
			return "", errors.New("server unavailable")
		}
		return backend, nil
	}
	write := func(_ context.Context, value string) error {
		events = append(events, "write:"+value)
		if !serverAvailable {
			return errors.New("server unavailable")
		}
		backend = value
		return nil
	}
	restart := func(context.Context) error {
		events = append(events, "restart")
		serverAvailable = true
		return nil
	}
	if err := restoreAdvancedBackend(context.Background(), "Legacy", read, write, restart); err != nil {
		t.Fatal(err)
	}
	if backend != "Legacy" {
		t.Fatalf("backend after independent cleanup = %q, want Legacy", backend)
	}
	if want := []string{"restart", "read", "write:Legacy", "restart", "read"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("cleanup events = %v, want %v", events, want)
	}
}

func TestAdvancedSessionEnumsValidateExactChoices(t *testing.T) {
	for name, want := range map[string][]string{
		"resume_data_storage_type":      {"Legacy", "SQLite"},
		"torrent_content_remove_option": {"Delete", "MoveToTrash"},
	} {
		field, ok := findPreferenceField(advancedSessionPreferenceFields, name)
		if !ok || !reflect.DeepEqual(field.stringEnums, want) {
			t.Fatalf("%s enum choices = %v, want %v", name, field.stringEnums, want)
		}
		for _, tc := range []struct {
			value string
			valid bool
		}{{want[0], true}, {want[1], true}, {"Invalid", false}} {
			var response validator.StringResponse
			(stringChoiceValidator{allowed: field.stringEnums}).ValidateString(ctx, validator.StringRequest{
				Path: path.Root(name), ConfigValue: types.StringValue(tc.value),
			}, &response)
			if got := !response.Diagnostics.HasError(); got != tc.valid {
				t.Errorf("%s=%q valid=%v want %v", name, tc.value, got, tc.valid)
			}
		}
	}
}

func TestAdvancedSessionInterfacePatchAndAddressReadback(t *testing.T) {
	for _, tc := range []struct {
		name          string
		inputAddress  string
		remoteAddress string
	}{
		{name: "canonicalized", inputAddress: " 192.0.2.17 ", remoteAddress: "192.0.2.17"},
		{name: "rejected", inputAddress: "not-an-address", remoteAddress: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			remote := map[string]string{
				"current_network_interface": "fixture-server-only0",
				"current_interface_address": "192.0.2.1",
				"current_interface_name":    "Fixture Interface",
			}
			writes := 0
			c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/app/setPreferences"):
					writes++
					if err := r.ParseForm(); err != nil {
						t.Errorf("parse patch: %v", err)
						return
					}
					var patch map[string]any
					if err := json.Unmarshal([]byte(r.Form.Get("json")), &patch); err != nil {
						t.Errorf("decode patch: %v", err)
						return
					}
					if len(patch) != 2 || patch["current_network_interface"] != "fixture-server-only0" || patch["current_interface_address"] != tc.inputAddress {
						t.Errorf("interface patch = %#v", patch)
					}
					remote["current_network_interface"] = patch["current_network_interface"].(string)
					remote["current_interface_address"] = tc.remoteAddress
				case strings.HasSuffix(r.URL.Path, "/app/preferences"):
					if err := json.NewEncoder(w).Encode(remote); err != nil {
						t.Errorf("write preferences response: %v", err)
					}
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			})
			planned := testState(t, prefsSchema(), map[string]attr.Value{
				"current_network_interface": types.StringValue("fixture-server-only0"),
				"current_interface_address": types.StringValue(tc.inputAddress),
			})
			r := &preferencesResource{client: c}
			out := resource.CreateResponse{State: tfsdk.State{Schema: planned.Schema}}
			r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: planned.Schema, Raw: planned.Raw}}, &out)
			if !out.Diagnostics.HasError() || writes != 1 {
				t.Fatalf("writes=%d diagnostics=%v; expected one write and a normalization diagnostic", writes, out.Diagnostics)
			}
			foundAddressDiagnostic := false
			for _, diagnostic := range out.Diagnostics.Errors() {
				withPath, hasPath := diagnostic.(interface{ Path() path.Path })
				if hasPath && diagnostic.Summary() == "Preference rejected or normalized" && withPath.Path().String() == "current_interface_address" {
					foundAddressDiagnostic = true
				}
			}
			if !foundAddressDiagnostic {
				t.Fatalf("missing address normalization/rejection diagnostic: %v", out.Diagnostics)
			}
			if got := attrString(t, out.State, "current_interface_address").ValueString(); got != tc.remoteAddress {
				t.Fatalf("state address=%q, want server readback %q", got, tc.remoteAddress)
			}
			if got := attrString(t, out.State, "current_network_interface").ValueString(); got != "fixture-server-only0" {
				t.Fatalf("state interface=%q", got)
			}
		})
	}
}
