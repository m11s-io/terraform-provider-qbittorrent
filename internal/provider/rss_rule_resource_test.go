package provider

import (
	"context"
	"encoding/json"
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
	if got["addPaused"] != nil || got["contentLayout"] != nil {
		t.Fatalf("null optional fields were not preserved: %#v", got)
	}
	if got["previouslyMatchedEpisodes"] == nil {
		t.Fatal("match history was reset on update")
	}
}

func TestRSSRuleDefinitionEncodesTypedNativeTorrentParams(t *testing.T) {
	typesByName := rssRuleTorrentParamAttributeTypes(context.Background())
	params, diags := types.ObjectValue(typesByName, map[string]attr.Value{
		"category": types.StringValue("tv"), "tags": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("weekly")}),
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
	if encoded["downloadLimit"] != int64(1048576) || encoded["ratioLimit"] != 1.75 || encoded["stopped"] != false || encoded["contentLayout"] != "Subfolder" {
		t.Fatalf("native AddTorrentParams not preserved: %#v", encoded)
	}
	if _, ok := encoded["downloadPath"]; ok {
		t.Fatalf("null optional path should remain unset: %#v", encoded)
	}
}

func TestRSSRuleDeprecatedAliasesAreAppliedAlongsideNativeParams(t *testing.T) {
	attributes := map[string]attr.Value{}
	for key, typ := range rssRuleTorrentParamAttributeTypes(context.Background()) {
		if _, ok := typ.(types.ListType); ok {
			attributes[key] = types.ListNull(types.StringType)
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
	if encoded["category"] != "edge" || encoded["savePath"] != "/server/path" || encoded["stopped"] != false || encoded["contentLayout"] != "Subfolder" {
		t.Fatalf("deprecated aliases were ignored when native params were present: %#v", encoded)
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
			if definition["addPaused"] != nil || definition["contentLayout"] != nil {
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
	remote := map[string]any{"enabled": true, "priority": float64(1), "useRegex": false, "mustContain": "old", "mustNotContain": "", "episodeFilter": "", "affectedFeeds": []any{"https://feed.invalid/rss"}, "lastMatch": "Fri, 30 Oct 2026 10:00:00 +0000", "ignoreDays": float64(0), "smartFilter": false, "previouslyMatchedEpisodes": []any{"S01E01"}, "addPaused": nil, "contentLayout": nil, "savePath": "", "assignedCategory": "", "torrentParams": map[string]any{"category": "", "tags": []any{}, "savePath": "", "downloadPath": "", "operatingMode": "AutoManaged", "seedMode": false, "uploadLimit": float64(-1), "downloadLimit": float64(-1), "ratioLimit": float64(0), "seedingTimeLimit": float64(-1), "inactiveSeedingTimeLimit": float64(-1), "shareLimitAction": "Default", "shareLimitsMode": "Default"}}
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
