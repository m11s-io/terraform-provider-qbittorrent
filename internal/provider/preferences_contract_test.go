package provider

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

// Compare independently maintained schema and registry contracts so a new field
// cannot silently disappear when the generic lifecycle encodes its typed patch.
func TestPreferenceRegistryMatchesGeneratedModels(t *testing.T) {
	read := preferenceJSONFields(reflect.TypeOf(client.Preferences{}))
	patch := preferenceJSONFields(reflect.TypeOf(client.PreferencesPatch{}))
	var rs resource.SchemaResponse
	(&preferencesResource{}).Schema(context.Background(), resource.SchemaRequest{}, &rs)
	var ds datasource.SchemaResponse
	(&preferencesDataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, &ds)
	kinds := map[preferenceKind]reflect.Kind{
		preferenceString: reflect.String, preferenceBool: reflect.Bool,
		preferenceInt: reflect.Int64, preferenceFloat: reflect.Float64,
	}
	for _, f := range allPreferenceFields() {
		t.Run(f.name, func(t *testing.T) {
			if got, ok := read[f.wireKey]; !ok || !preferenceModelKindMatches(f, got, kinds[f.kind]) {
				t.Errorf("response model type for %s: %v, want %v", f.wireKey, got, kinds[f.kind])
			}
			pt, writable := patch[f.wireKey]
			if f.readOnly {
				if writable || rs.Schema.Attributes[f.name] != nil {
					t.Error("data-source-only field has writable ownership")
				}
			} else if !writable || !preferenceModelKindMatches(f, pt, kinds[f.kind]) || rs.Schema.Attributes[f.name] == nil {
				t.Errorf("missing or mistyped writable contract: %s (%v)", f.wireKey, pt)
			}
			_, exposed := ds.Schema.Attributes[f.name]
			if exposed == f.excludeFromDataSource {
				t.Error("data-source exposure differs from field metadata")
			}
		})
	}
	for _, key := range []string{"mail_notification_password", "proxy_password", "dyndns_password", "web_ui_password", "web_ui_api_key"} {
		if _, ok := read[key]; ok {
			t.Errorf("credential %s appears in public response model", key)
		}
	}
	for _, key := range []string{"scan_dirs", "web_ui_address", "web_ui_port", "use_https", "web_ui_https_cert_path", "web_ui_https_key_path", "web_ui_username", "web_ui_password"} {
		for _, f := range allPreferenceFields() {
			if f.wireKey == key {
				t.Errorf("specialized resource key %s has generic preference ownership", key)
			}
		}
	}
}

func preferenceJSONFields(model reflect.Type) map[string]reflect.Type {
	fields := map[string]reflect.Type{}
	for i := 0; i < model.NumField(); i++ {
		f := model.Field(i)
		key := strings.Split(f.Tag.Get("json"), ",")[0]
		if key == "" || key == "-" {
			continue
		}
		typ := f.Type
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		fields[key] = typ
	}
	return fields
}

func preferenceModelKindMatches(f preferenceField, typ reflect.Type, want reflect.Kind) bool {
	if typ.Kind() == want {
		return true
	}
	if f.kind != preferenceInt || (typ.Kind() != reflect.Int && typ.Kind() != reflect.Int32) {
		return false
	}
	// Native 32-bit counters and bounded enums need not become int64 in the
	// transport model. Large/unbounded byte counters must remain int64.
	fits32 := func(n int64) bool { return n >= -2147483648 && n <= 2147483647 }
	for _, n := range f.integerSentinels {
		if !fits32(n) {
			return false
		}
	}
	if len(f.integerEnums) > 0 {
		for _, n := range f.integerEnums {
			if !fits32(n) {
				return false
			}
		}
		return true
	}
	return f.integerMin != nil && f.integerMax != nil && fits32(*f.integerMin) && fits32(*f.integerMax)
}
