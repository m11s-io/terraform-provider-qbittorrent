package provider

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPreferenceValuePreservesFractionalRatioAndZero(t *testing.T) {
	for _, wire := range []string{"1.75", "0"} {
		actual, err := preferenceValue(types.Float64Value(0), []byte(wire))
		if err != nil {
			t.Fatal(err)
		}
		value, ok := actual.(types.Float64)
		if !ok || value.ValueFloat64() != mustParseFloat(wire) {
			t.Fatalf("want exact Float64 %s, got %#v", wire, actual)
		}
	}
}

func TestFloatPreferenceValidatorRequiresFiniteValueAndHonorsBounds(t *testing.T) {
	f := field("max_ratio", "BitTorrent", "float", "Share ratio")
	f.floatMin, f.floatMax = float64ptr(-1), float64ptr(1000)
	for _, tc := range []struct {
		value float64
		valid bool
	}{
		{value: 1.75, valid: true}, {value: -1, valid: true}, {value: 1001}, {value: math.NaN()}, {value: math.Inf(1)},
	} {
		if got := validPreferenceFloat(f, tc.value); got != tc.valid {
			t.Errorf("value %v: valid=%v want %v", tc.value, got, tc.valid)
		}
		if math.IsNaN(tc.value) || math.IsInf(tc.value, 0) {
			continue
		}
		var resp validator.Float64Response
		(preferenceFloatValidator{field: f}).ValidateFloat64(ctx, validator.Float64Request{Path: path.Root(f.name), ConfigValue: types.Float64Value(tc.value)}, &resp)
		if resp.Diagnostics.HasError() == tc.valid {
			t.Errorf("value %v: diagnostics=%v want valid=%v", tc.value, resp.Diagnostics, tc.valid)
		}
	}
}

func mustParseFloat(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }

func TestPreferenceDataSourceAllowsMissingCapabilityFieldOnly(t *testing.T) {
	optional := field("maybe", "Advanced", "int", "optional runtime field")
	optional.capability = "optional-build-feature"
	value, err := preferenceDataSourceValue(optional, nil, false)
	if err != nil || !value.IsNull() {
		t.Fatalf("optional unsupported field should be null: %v %v", value, err)
	}
	established := field("known", "Behavior", "int", "established field")
	if _, err := preferenceDataSourceValue(established, nil, false); err == nil {
		t.Fatal("missing established field should remain an error")
	}
	value, err = preferenceDataSourceValue(optional, json.RawMessage("7"), true)
	if err != nil || value.(types.Int64).ValueInt64() != 7 {
		t.Fatalf("supported value did not decode: %v %v", value, err)
	}
}

func TestPreferenceFieldsHaveDeterministicUniqueWireOwnership(t *testing.T) {
	fields := allPreferenceFields()
	if len(fields) == 0 {
		t.Fatal("allPreferenceFields returned no fields")
	}
	if err := validatePreferenceFields(fields); err != nil {
		t.Fatalf("invalid aggregate preference fields: %v", err)
	}
	seen := map[string]string{}
	for _, field := range fields {
		if field.name == "" || field.wireKey == "" || field.tab == "" || field.description == "" {
			t.Errorf("incomplete preference metadata: %+v", field)
		}
		if previous, ok := seen[field.wireKey]; ok {
			t.Errorf("wire key %q is owned by both %q and %q", field.wireKey, previous, field.name)
		}
		seen[field.wireKey] = field.name
	}
	second := allPreferenceFields()
	if len(second) != len(fields) {
		t.Fatalf("field count changed between calls: %d then %d", len(fields), len(second))
	}
	for i := range fields {
		if fields[i].name != second[i].name || fields[i].wireKey != second[i].wireKey {
			t.Fatalf("field order changed at %d: %+v then %+v", i, fields[i], second[i])
		}
	}
}

func TestPreferenceFieldsRejectDuplicateWireOwnership(t *testing.T) {
	fields := []preferenceField{
		{name: "first", wireKey: "same", kind: preferenceString, tab: "One", description: "first"},
		{name: "second", wireKey: "same", kind: preferenceBool, tab: "Two", description: "second"},
	}
	if err := validatePreferenceFields(fields); err == nil {
		t.Fatal("expected duplicate wire ownership to fail")
	}
}

func TestLegacyRateResourceDescriptionsRetainCompatibilityText(t *testing.T) {
	for _, name := range []string{"dl_limit", "up_limit", "alt_dl_limit", "alt_up_limit"} {
		f, ok := findPreferenceField(legacyPreferenceFields, name)
		if !ok {
			t.Fatalf("missing legacy field %s", name)
		}
		want := f.description + " Rate limits must be multiples of 1024, at most 2147482624."
		if got := preferenceResourceDescription(f); got != want {
			t.Errorf("%s description = %q want %q", name, got, want)
		}
	}
}

func TestReadOnlyPreferenceNeverEntersGenericPatch(t *testing.T) {
	f := field("resolved_name", "Advanced", "string", "Display-only server value")
	f.readOnly = true
	wire, value, include := preferencePatchEntry([]preferenceField{f}, f.name, "eth0")
	if include || wire != "" || value != nil {
		t.Fatalf("read-only output entered patch: %q %#v %v", wire, value, include)
	}
	wire, value, include = preferencePatchEntry([]preferenceField{field("persisted", "Advanced", "string", "Writable server value")}, "persisted", "x")
	if !include || wire != "persisted" || value != "x" {
		t.Fatalf("ordinary field missing from patch: %q %#v %v", wire, value, include)
	}
}
