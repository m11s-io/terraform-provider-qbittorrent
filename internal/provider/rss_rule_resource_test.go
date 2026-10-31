package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

func rssRuleSchema() rs.Schema {
	var out resource.SchemaResponse
	(&rssRuleResource{}).Schema(context.Background(), resource.SchemaRequest{}, &out)
	return out.Schema
}

func rssRuleState(t *testing.T, s rs.Schema, values map[string]attr.Value) tfsdk.State {
	t.Helper()
	attrTypes := s.Type().(types.ObjectType).AttrTypes
	raw := map[string]tftypes.Value{}
	for name, typ := range attrTypes {
		var v tftypes.Value
		switch typ := typ.(type) {
		case basetypes.StringType:
			v = tftypes.NewValue(tftypes.String, nil)
		case basetypes.BoolType:
			v = tftypes.NewValue(tftypes.Bool, nil)
		case basetypes.Int64Type:
			v = tftypes.NewValue(tftypes.Number, nil)
		case basetypes.Float64Type:
			v = tftypes.NewValue(tftypes.Number, nil)
		case types.ListType:
			v = tftypes.NewValue(typ.TerraformType(context.Background()), nil)
		case types.SetType:
			v = tftypes.NewValue(typ.TerraformType(context.Background()), nil)
		case types.ObjectType:
			v = tftypes.NewValue(typ.TerraformType(context.Background()), nil)
		default:
			t.Fatalf("unsupported test attr %s type %T", name, typ)
		}
		if value, ok := values[name]; ok {
			tv, err := value.ToTerraformValue(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			v = tv
		}
		raw[name] = v
	}
	state := tfsdk.State{Schema: s}
	d := state.Set(context.Background(), tftypes.NewValue(s.Type().TerraformType(context.Background()), raw))
	if d.HasError() {
		t.Fatal(d)
	}
	return state
}

func TestRSSRuleSchemaHasComputedHistoryAndTypedFields(t *testing.T) {
	s := rssRuleSchema()
	for _, name := range []string{"enabled", "priority", "use_regex", "must_contain", "must_not_contain", "episode_filter", "affected_feeds", "ignore_days", "smart_filter", "add_stopped", "content_layout", "torrent_params"} {
		if _, ok := s.Attributes[name]; !ok {
			t.Errorf("missing rule field %s", name)
		}
	}
	if !s.Attributes["last_match"].IsComputed() || !s.Attributes["previously_matched_episodes"].IsComputed() {
		t.Fatal("runtime match history must be computed")
	}
	name, ok := s.Attributes["rule_name"].(rs.StringAttribute)
	if !ok || !name.Required || len(name.PlanModifiers) != 1 {
		t.Fatal("rule name must be required and changing it must replace the resource")
	}
	if _, ok := s.Attributes["rule"].(rs.MapAttribute); ok {
		t.Fatal("generic JSON rule escape hatch is not allowed")
	}
	nested := s.Attributes["torrent_params"].(rs.SingleNestedAttribute)
	if _, ok := nested.Attributes["tags"].(rs.SetAttribute); !ok {
		t.Fatalf("torrent_params.tags must have unordered set semantics, got %T", nested.Attributes["tags"])
	}
}

func TestRSSRuleUpdateRefusesExistingTLSMaterialAndAllowsEmptyTLSFields(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tls      map[string]any
		wantPost bool
	}{
		{name: "certificate", tls: map[string]any{"ssl_certificate": "CERT_CANARY_91a5"}},
		{name: "private key", tls: map[string]any{"ssl_private_key": "KEY_CANARY_52bf"}},
		{name: "dh params", tls: map[string]any{"ssl_dh_params": "DH_CANARY_f131"}},
		{name: "empty TLS fields", tls: map[string]any{"ssl_certificate": "", "ssl_private_key": "", "ssl_dh_params": ""}, wantPost: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			remote := map[string]any{"enabled": true, "mustContain": "old", "torrentParams": tc.tls}
			c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/rss/rules"):
					_ = json.NewEncoder(w).Encode(map[string]any{"Weekly": remote})
				case strings.HasSuffix(r.URL.Path, "/rss/setRule"):
					posts++
				}
			})
			s := rssRuleSchema()
			r := &rssRuleResource{client: c}
			prior := rssRuleState(t, s, map[string]attr.Value{"id": types.StringValue("Weekly"), "rule_name": types.StringValue("Weekly"), "enabled": types.BoolValue(true), "must_contain": types.StringValue("old")})
			plan := rssRuleState(t, s, map[string]attr.Value{"id": types.StringValue("Weekly"), "rule_name": types.StringValue("Weekly"), "enabled": types.BoolValue(true), "must_contain": types.StringValue("new")})
			var update resource.UpdateResponse
			update.State = prior
			r.Update(context.Background(), resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s, Raw: plan.Raw}, State: prior}, &update)
			if (posts != 0) != tc.wantPost {
				t.Fatalf("setRule POST count = %d, wantPost=%v", posts, tc.wantPost)
			}
			if tc.wantPost {
				if update.Diagnostics.HasError() {
					t.Fatalf("empty TLS fields should allow unrelated update: %v", update.Diagnostics)
				}
				return
			}
			if !update.Diagnostics.HasError() {
				t.Fatal("update with unsupported TLS material should be rejected")
			}
			if strings.Contains(fmt.Sprint(update.Diagnostics), "CANARY") {
				t.Fatalf("diagnostic exposed secret material: %v", update.Diagnostics)
			}
		})
	}
}

func TestRSSRuleDefinitionPreservesHistoryAndOptionalNulls(t *testing.T) {
	m := rssRuleModel{RuleName: types.StringValue("Weekly"), Enabled: types.BoolValue(false), Priority: types.Int64Value(4), UseRegex: types.BoolValue(true),
		MustContain: types.StringValue("S01"), MustNotContain: types.StringValue("sample"), EpisodeFilter: types.StringValue("S\\d+E\\d+"),
		AffectedFeeds: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("https://feed.invalid/rss")}), IgnoreDays: types.Int64Value(2), SmartFilter: types.BoolValue(true),
		AddStopped: types.BoolNull(), ContentLayout: types.StringNull(), TorrentParams: types.ObjectNull(rssRuleTorrentParamAttributeTypes(context.Background()))}
	history := client.RSSRule{"lastMatch": "Fri, 30 Oct 2026 10:00:00 +0000", "previouslyMatchedEpisodes": []any{"S01E01"}}
	got, err := rssRuleDefinition(context.Background(), m, history)
	if err != nil {
		t.Fatal(err)
	}
	if got["enabled"] != false || got["priority"] != int64(4) || got["lastMatch"] != history["lastMatch"] {
		t.Fatalf("wrong generated rule: %#v", got)
	}
	if got["addPaused"] != nil || got["torrentContentLayout"] != nil {
		t.Fatalf("null optional fields were not preserved: %#v", got)
	}
	if got["previouslyMatchedEpisodes"] == nil {
		t.Fatal("match history was reset on update")
	}
}

func TestRSSRuleDefinitionEncodesTypedNativeTorrentParams(t *testing.T) {
	typesByName := rssRuleTorrentParamAttributeTypes(context.Background())
	params, diags := types.ObjectValue(typesByName, map[string]attr.Value{
		"category": types.StringValue("tv"), "tags": types.SetValueMust(types.StringType, []attr.Value{types.StringValue("weekly")}),
		"save_path": types.StringValue("/srv/downloads"), "download_path": types.StringNull(), "use_download_path": types.BoolNull(),
		"operating_mode": types.StringNull(), "seed_mode": types.BoolValue(false), "upload_limit": types.Int64Value(0), "download_limit": types.Int64Value(1048576),
		"ratio_limit": types.Float64Value(1.75), "seeding_time_limit": types.Int64Value(-1), "inactive_seeding_time_limit": types.Int64Value(0),
		"share_limit_action": types.StringNull(), "share_limits_mode": types.StringNull(), "queue_top": types.BoolNull(), "stopped": types.BoolValue(false),
		"stop_condition": types.StringNull(), "content_layout": types.StringValue("Subfolder"), "auto_tmm": types.BoolValue(false),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	m := rssRuleModel{RuleName: types.StringValue("Weekly"), TorrentParams: params, AddStopped: types.BoolNull(), ContentLayout: types.StringNull()}
	got, err := rssRuleDefinition(context.Background(), m, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded := got["torrentParams"].(map[string]any)
	if encoded["download_limit"] != int64(1048576) || encoded["ratio_limit"] != 1.75 || encoded["stopped"] != false || encoded["content_layout"] != "Subfolder" {
		t.Fatalf("native AddTorrentParams not preserved: %#v", encoded)
	}
	if _, ok := encoded["download_path"]; ok {
		t.Fatalf("null optional path should remain unset: %#v", encoded)
	}
}

func TestRSSRuleTagSetConversionAndRefreshUsesNativeSetSemantics(t *testing.T) {
	attrs := map[string]attr.Value{}
	for key, typ := range rssRuleTorrentParamAttributeTypes(context.Background()) {
		switch typ {
		case types.StringType:
			attrs[key] = types.StringNull()
		case types.BoolType:
			attrs[key] = types.BoolNull()
		case types.Int64Type:
			attrs[key] = types.Int64Null()
		case types.Float64Type:
			attrs[key] = types.Float64Null()
		default:
			if key == "tags" {
				attrs[key] = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("z"), types.StringValue("a"), types.StringValue("z")})
			} else {
				t.Fatalf("unexpected torrent param type %s: %T", key, typ)
			}
		}
	}
	params, diags := types.ObjectValue(rssRuleTorrentParamAttributeTypes(context.Background()), attrs)
	if diags.HasError() {
		t.Fatal(diags)
	}
	m := rssRuleModel{RuleName: types.StringValue("Weekly"), TorrentParams: params}
	definition, err := rssRuleDefinition(context.Background(), m, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded := definition["torrentParams"].(map[string]any)["tags"].([]any)
	if len(encoded) != 2 {
		t.Fatalf("set conversion did not deduplicate tags: %#v", encoded)
	}
	gotSet := map[string]bool{}
	for _, raw := range encoded {
		gotSet[raw.(string)] = true
	}
	if !gotSet["z"] || !gotSet["a"] {
		t.Fatalf("set conversion lost an unordered tag: %#v", encoded)
	}
	rssRuleRefreshConfigured(&m, client.RSSRule{"torrentParams": map[string]any{"tags": []any{"z", "a", "z"}}})
	refreshed := m.TorrentParams.Attributes()["tags"].(types.Set)
	var tags []string
	if d := refreshed.ElementsAs(context.Background(), &tags, false); d.HasError() {
		t.Fatal(d)
	}
	if len(tags) != 2 {
		t.Fatalf("native response was not normalized to two distinct tags: %#v", tags)
	}
	refreshedSet := map[string]bool{}
	for _, tag := range tags {
		refreshedSet[tag] = true
	}
	if !refreshedSet["z"] || !refreshedSet["a"] {
		t.Fatalf("native response lost a tag during refresh: %#v", tags)
	}
}

func TestRSSRuleDeprecatedAliasesAreAppliedAlongsideNativeParams(t *testing.T) {
	attributes := map[string]attr.Value{}
	for key, typ := range rssRuleTorrentParamAttributeTypes(context.Background()) {
		if key == "tags" {
			attributes[key] = types.SetNull(types.StringType)
			continue
		}
		switch typ {
		case types.StringType:
			attributes[key] = types.StringNull()
		case types.BoolType:
			attributes[key] = types.BoolNull()
		case types.Int64Type:
			attributes[key] = types.Int64Null()
		case types.Float64Type:
			attributes[key] = types.Float64Null()
		}
	}
	attributes["content_layout"] = types.StringValue("NoSubfolder")
	params, diags := types.ObjectValue(rssRuleTorrentParamAttributeTypes(context.Background()), attributes)
	if diags.HasError() {
		t.Fatal(diags)
	}
	m := rssRuleModel{RuleName: types.StringValue("Weekly"), TorrentParams: params, Category: types.StringValue("edge"), SavePath: types.StringValue("/server/path"), AddStopped: types.BoolValue(false), ContentLayout: types.StringValue("Subfolder")}
	got, err := rssRuleDefinition(context.Background(), m, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded := got["torrentParams"].(map[string]any)
	if encoded["category"] != "edge" || encoded["save_path"] != "/server/path" || encoded["stopped"] != false || encoded["content_layout"] != "NoSubfolder" {
		t.Fatalf("deprecated aliases were ignored when native params were present: %#v", encoded)
	}
	if got["torrentContentLayout"] != "Subfolder" {
		t.Fatalf("legacy layout alias used wrong native key: %#v", got)
	}
}

func TestRSSRuleRefreshPreservesNullOptionalTorrentParams(t *testing.T) {
	attributes := map[string]attr.Value{}
	for key, typ := range rssRuleTorrentParamAttributeTypes(context.Background()) {
		if key == "tags" {
			attributes[key] = types.SetNull(types.StringType)
			continue
		}
		switch typ {
		case types.StringType:
			attributes[key] = types.StringNull()
		case types.BoolType:
			attributes[key] = types.BoolNull()
		case types.Int64Type:
			attributes[key] = types.Int64Null()
		case types.Float64Type:
			attributes[key] = types.Float64Null()
		}
	}
	attributes["stopped"] = types.BoolValue(true)
	attributes["content_layout"] = types.StringValue("Subfolder")
	params, diags := types.ObjectValue(rssRuleTorrentParamAttributeTypes(context.Background()), attributes)
	if diags.HasError() {
		t.Fatal(diags)
	}
	m := rssRuleModel{TorrentParams: params}
	rssRuleRefreshConfigured(&m, client.RSSRule{"torrentParams": map[string]any{
		"category": "", "tags": []any{}, "save_path": "", "download_path": "", "operating_mode": "AutoManaged", "seed_mode": false,
		"upload_limit": float64(-1), "download_limit": float64(-1), "ratio_limit": float64(0), "seeding_time_limit": float64(-1),
		"inactive_seeding_time_limit": float64(-1), "share_limit_action": "Default", "share_limits_mode": "Default", "stopped": true, "content_layout": "Subfolder",
	}})
	got := m.TorrentParams.Attributes()
	for _, name := range []string{"category", "tags", "save_path", "download_path", "operating_mode", "upload_limit", "download_limit"} {
		if !got[name].IsNull() {
			t.Errorf("unconfigured nested attribute %s became non-null: %v", name, got[name])
		}
	}
	if got["stopped"].(types.Bool).ValueBool() != true || got["content_layout"].(types.String).ValueString() != "Subfolder" {
		t.Fatalf("configured nested attributes changed: %#v", got)
	}
}

func TestRSSRuleRefreshClearsConfiguredOptionalParamsWhenNativeOmitsThem(t *testing.T) {
	attributes := map[string]attr.Value{}
	for key, typ := range rssRuleTorrentParamAttributeTypes(context.Background()) {
		if key == "tags" {
			attributes[key] = types.SetNull(types.StringType)
			continue
		}
		switch typ {
		case types.StringType:
			attributes[key] = types.StringNull()
		case types.BoolType:
			attributes[key] = types.BoolNull()
		case types.Int64Type:
			attributes[key] = types.Int64Null()
		case types.Float64Type:
			attributes[key] = types.Float64Null()
		}
	}
	attributes["stopped"] = types.BoolValue(true)
	attributes["content_layout"] = types.StringValue("Subfolder")
	attributes["auto_tmm"] = types.BoolValue(false)
	attributes["use_download_path"] = types.BoolValue(false)
	attributes["queue_top"] = types.BoolValue(true)
	attributes["stop_condition"] = types.StringValue("FilesChecked")
	params, diags := types.ObjectValue(rssRuleTorrentParamAttributeTypes(context.Background()), attributes)
	if diags.HasError() {
		t.Fatal(diags)
	}
	m := rssRuleModel{TorrentParams: params}
	rssRefreshTorrentParams(&m, map[string]any{
		"use_auto_tmm": nil, "use_download_path": nil, "stop_condition": nil,
	})
	got := m.TorrentParams.Attributes()
	for _, name := range []string{"stopped", "content_layout", "auto_tmm", "use_download_path", "queue_top", "stop_condition"} {
		if !got[name].IsNull() {
			t.Errorf("configured optional field %s was silently retained after native omission/null: %v", name, got[name])
		}
	}
	for _, name := range []string{"category", "tags", "save_path", "download_limit"} {
		if !got[name].IsNull() {
			t.Errorf("unconfigured optional field %s must remain null: %v", name, got[name])
		}
	}
}

func TestRSSRuleUpdateMergesCurrentNativeParamsAndConfiguredFieldsWin(t *testing.T) {
	attributes := map[string]attr.Value{}
	for key, typ := range rssRuleTorrentParamAttributeTypes(context.Background()) {
		if key == "tags" {
			attributes[key] = types.SetNull(types.StringType)
			continue
		}
		switch typ {
		case types.StringType:
			attributes[key] = types.StringNull()
		case types.BoolType:
			attributes[key] = types.BoolNull()
		case types.Int64Type:
			attributes[key] = types.Int64Null()
		case types.Float64Type:
			attributes[key] = types.Float64Null()
		}
	}
	attributes["stopped"] = types.BoolValue(true)
	attributes["content_layout"] = types.StringValue("NoSubfolder")
	params, diags := types.ObjectValue(rssRuleTorrentParamAttributeTypes(context.Background()), attributes)
	if diags.HasError() {
		t.Fatal(diags)
	}
	m := rssRuleModel{RuleName: types.StringValue("Weekly"), TorrentParams: params, AddStopped: types.BoolValue(false), ContentLayout: types.StringValue("Subfolder"), SavePath: types.StringValue("/new/path")}
	history := client.RSSRule{"torrentParams": map[string]any{
		"category": "existing", "tags": []any{"keep"}, "save_path": "/old/path", "download_path": "/old/download", "operating_mode": "Forced", "seed_mode": true,
		"upload_limit": float64(11), "download_limit": float64(22), "ratio_limit": 1.5, "seeding_time_limit": float64(33), "inactive_seeding_time_limit": float64(44),
		"share_limit_action": "Remove", "share_limits_mode": "MatchAll", "add_to_top_of_queue": true, "stopped": false, "stop_condition": "FilesChecked", "content_layout": "Original", "use_auto_tmm": false,
	}}
	got, err := rssRuleDefinition(context.Background(), m, history)
	if err != nil {
		t.Fatal(err)
	}
	encoded := got["torrentParams"].(map[string]any)
	if encoded["category"] != "existing" || encoded["tags"].([]any)[0] != "keep" || encoded["download_path"] != "/old/download" || encoded["operating_mode"] != "Forced" {
		t.Fatalf("partial update dropped existing native params: %#v", encoded)
	}
	if encoded["save_path"] != "/new/path" || encoded["stopped"] != true || encoded["content_layout"] != "NoSubfolder" {
		t.Fatalf("configured aliases/values not applied to native params: %#v", encoded)
	}
	if got["torrentContentLayout"] != "Subfolder" || got["addPaused"] != false {
		t.Fatalf("legacy values not retained separately: %#v", got)
	}
}

func TestRSSRuleCreateRefusesDuplicateInsteadOfAdoptingOrOverwriting(t *testing.T) {
	writes := 0
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/rss/rules") {
			_, _ = w.Write([]byte(`{"Weekly":{"enabled":true}}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/rss/setRule") {
			writes++
		}
	})
	s := rssRuleSchema()
	state := rssRuleState(t, s, map[string]attr.Value{"rule_name": types.StringValue("Weekly")})
	r := &rssRuleResource{client: c}
	var resp resource.CreateResponse
	resp.State = state
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: state.Raw}}, &resp)
	if !resp.Diagnostics.HasError() || writes != 0 {
		t.Fatalf("duplicate rule should require import, diagnostics=%v writes=%d", resp.Diagnostics, writes)
	}
}

func TestRSSRuleCreateWritesOnlyOneRuleAndDeleteRemovesOnlyThatRule(t *testing.T) {
	remote := map[string]any{}
	deleted := ""
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/rss/rules"):
			_ = json.NewEncoder(w).Encode(remote)
		case strings.HasSuffix(r.URL.Path, "/rss/setRule"):
			if r.Method != http.MethodPost {
				t.Errorf("setRule method = %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if len(r.Form) != 2 || r.Form.Get("ruleName") != "Local rule" {
				t.Errorf("wrong set form: %v", r.Form)
			}
			var definition map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("ruleDef")), &definition); err != nil {
				t.Fatal(err)
			}
			if definition["addPaused"] != nil || definition["torrentContentLayout"] != nil {
				t.Errorf("optional null aliases lost: %#v", definition)
			}
			if definition["mustContain"] != "S01" || definition["enabled"] != false {
				t.Errorf("wrong rule payload: %#v", definition)
			}
			remote["Local rule"] = definition
		case strings.HasSuffix(r.URL.Path, "/rss/removeRule"):
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if len(r.Form) != 1 {
				t.Errorf("remove form has unrelated fields: %v", r.Form)
			}
			deleted = r.Form.Get("ruleName")
			delete(remote, deleted)
		}
	})
	s := rssRuleSchema()
	r := &rssRuleResource{client: c}
	plan := rssRuleState(t, s, map[string]attr.Value{
		"rule_name": types.StringValue("Local rule"), "enabled": types.BoolValue(false), "priority": types.Int64Value(3),
		"must_contain": types.StringValue("S01"), "affected_feeds": types.ListValueMust(types.StringType, []attr.Value{}),
	})
	var create resource.CreateResponse
	create.State = plan
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: plan.Raw}}, &create)
	if create.Diagnostics.HasError() {
		t.Fatal(create.Diagnostics)
	}
	if len(remote) != 1 {
		t.Fatalf("create modified more than one rule: %#v", remote)
	}
	var id types.String
	if d := create.State.GetAttribute(context.Background(), path.Root("id"), &id); d.HasError() || id.ValueString() != "Local rule" {
		t.Fatalf("resource ID = %q, %v", id.ValueString(), d)
	}
	var remove resource.DeleteResponse
	r.Delete(context.Background(), resource.DeleteRequest{State: create.State}, &remove)
	if remove.Diagnostics.HasError() || deleted != "Local rule" || len(remote) != 0 {
		t.Fatalf("delete affected wrong objects: deleted=%q rules=%#v diagnostics=%v", deleted, remote, remove.Diagnostics)
	}
}

func TestRSSRuleImportUsesExactRuleName(t *testing.T) {
	s := rssRuleSchema()
	r := &rssRuleResource{}
	var resp resource.ImportStateResponse
	resp.State = rssRuleState(t, s, nil)
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "Weekly Release"}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var imported types.String
	if d := resp.State.GetAttribute(context.Background(), path.Root("rule_name"), &imported); d.HasError() || imported.ValueString() != "Weekly Release" {
		t.Fatalf("imported rule name = %q, %v", imported.ValueString(), d)
	}
}

func TestRSSRuleUpdateCarriesNativeHistoryAndRefreshRemovesMissingRule(t *testing.T) {
	remote := map[string]any{"enabled": true, "priority": float64(1), "useRegex": false, "mustContain": "old", "mustNotContain": "", "episodeFilter": "", "affectedFeeds": []any{"https://feed.invalid/rss"}, "lastMatch": "Fri, 30 Oct 2026 10:00:00 +0000", "ignoreDays": float64(0), "smartFilter": false, "previouslyMatchedEpisodes": []any{"S01E01"}, "addPaused": nil, "torrentContentLayout": nil, "savePath": "", "assignedCategory": "", "torrentParams": map[string]any{"category": "", "tags": []any{}, "save_path": "", "download_path": "", "operating_mode": "AutoManaged", "seed_mode": false, "upload_limit": float64(-1), "download_limit": float64(-1), "ratio_limit": float64(0), "seeding_time_limit": float64(-1), "inactive_seeding_time_limit": float64(-1), "share_limit_action": "Default", "share_limits_mode": "Default", "stopped": nil, "content_layout": nil, "use_auto_tmm": nil, "add_to_top_of_queue": nil}}
	deleted := false
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/rss/rules"):
			if deleted {
				_, _ = w.Write([]byte(`{}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"Weekly": remote})
		case strings.HasSuffix(r.URL.Path, "/rss/setRule"):
			_ = r.ParseForm()
			var got map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("ruleDef")), &got); err != nil {
				t.Fatal(err)
			}
			if got["lastMatch"] != remote["lastMatch"] || len(got["previouslyMatchedEpisodes"].([]any)) != 1 {
				t.Errorf("update reset runtime history: %#v", got)
			}
			remote = got
		case strings.HasSuffix(r.URL.Path, "/rss/removeRule"):
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if len(r.Form) != 1 || r.Form.Get("ruleName") != "Weekly" {
				t.Errorf("delete form is unsafe: %v", r.Form)
			}
			deleted = true
		}
	})
	s := rssRuleSchema()
	r := &rssRuleResource{client: c}
	prior := rssRuleState(t, s, map[string]attr.Value{"id": types.StringValue("Weekly"), "rule_name": types.StringValue("Weekly"), "enabled": types.BoolValue(true), "must_contain": types.StringValue("old")})
	plan := rssRuleState(t, s, map[string]attr.Value{"id": types.StringValue("Weekly"), "rule_name": types.StringValue("Weekly"), "enabled": types.BoolValue(true), "must_contain": types.StringValue("new")})
	var update resource.UpdateResponse
	update.State = prior
	r.Update(context.Background(), resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s, Raw: plan.Raw}, State: prior}, &update)
	if update.Diagnostics.HasError() {
		t.Fatal(update.Diagnostics)
	}
	got := update.State
	var last types.String
	if d := got.GetAttribute(context.Background(), path.Root("last_match"), &last); d.HasError() || last.ValueString() != remote["lastMatch"] {
		t.Fatalf("history state = %q, %v", last.ValueString(), d)
	}

	remote["mustContain"] = "server drift"
	var read resource.ReadResponse
	read.State = got
	r.Read(context.Background(), resource.ReadRequest{State: got}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	var drifted types.String
	if d := read.State.GetAttribute(context.Background(), path.Root("must_contain"), &drifted); d.HasError() || drifted.ValueString() != "server drift" {
		t.Fatalf("refresh did not detect rule drift: %q, %v", drifted.ValueString(), d)
	}
	deleted = true
	var missing resource.ReadResponse
	r.Read(context.Background(), resource.ReadRequest{State: read.State}, &missing)
	if !missing.State.Raw.IsNull() {
		t.Fatal("missing remote rule was not removed from state")
	}

	var remove resource.DeleteResponse
	r.Delete(context.Background(), resource.DeleteRequest{State: got}, &remove)
	if remove.Diagnostics.HasError() {
		t.Fatal(remove.Diagnostics)
	}
}
