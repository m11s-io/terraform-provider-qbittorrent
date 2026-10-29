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

func TestSpeedPreferencesLifecycle(t *testing.T) {
	want := map[string]any{
		"scheduler_enabled": false, "schedule_from_hour": float64(0), "schedule_from_min": float64(0),
		"schedule_to_hour": float64(23), "schedule_to_min": float64(59), "scheduler_days": float64(9),
		"limit_utp_rate": false, "limit_tcp_overhead": true, "limit_lan_peers": false,
	}
	fields := map[string]attr.Value{}
	for k, v := range want {
		switch v := v.(type) {
		case bool:
			fields[k] = types.BoolValue(v)
		case float64:
			fields[k] = types.Int64Value(int64(v))
		}
	}
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
				t.Fatalf("unexpected partial update: %v", got)
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("%s: got %v want %v", k, got[k], v)
				}
			}
			return
		}
		json.NewEncoder(w).Encode(want)
	})
	r := &preferencesResource{client: c}
	s := testState(t, prefsSchema(), fields)
	var out resource.CreateResponse
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}}, &out)
	if out.Diagnostics.HasError() || writes != 1 {
		t.Fatalf("writes=%d diagnostics=%v", writes, out.Diagnostics)
	}
	for k, v := range fields {
		var got attr.Value
		// Read the object to assert the managed state retains false and zero.
		var obj types.Object
		out.State.Get(ctx, &obj)
		got = obj.Attributes()[k]
		if !v.Equal(got) {
			t.Errorf("state %s: got %v want %v", k, got, v)
		}
	}
}

func TestSpeedPreferenceRanges(t *testing.T) {
	s := prefsSchema()
	for _, tc := range []struct {
		name string
		max  int64
	}{
		{"schedule_from_hour", 23}, {"schedule_to_hour", 23},
		{"schedule_from_min", 59}, {"schedule_to_min", 59}, {"scheduler_days", 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, ok := s.Attributes[tc.name].(interface{ Int64Validators() []validator.Int64 })
			if !ok {
				t.Fatal("missing numeric preference")
			}
			for _, n := range []int64{-1, 0, tc.max, tc.max + 1} {
				var resp validator.Int64Response
				for _, v := range a.Int64Validators() {
					v.ValidateInt64(ctx, validator.Int64Request{Path: path.Root(tc.name), ConfigValue: types.Int64Value(n)}, &resp)
				}
				if resp.Diagnostics.HasError() != (n < 0 || n > tc.max) {
					t.Errorf("value %d: %v", n, resp.Diagnostics)
				}
			}
		})
	}
}

func TestSchedulePairs(t *testing.T) {
	for _, prefix := range []string{"schedule_from_", "schedule_to_"} {
		for _, part := range []string{"hour", "min"} {
			s := testState(t, prefsSchema(), map[string]attr.Value{prefix + part: types.Int64Value(0)})
			var resp resource.ValidateConfigResponse
			(&preferencesResource{}).ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: s.Schema, Raw: s.Raw}}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Errorf("accepted unpaired %s%s", prefix, part)
			}
		}
	}
}
