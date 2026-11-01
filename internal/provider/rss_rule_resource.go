package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

type rssRuleResource struct{ client *client.Client }

type rssRuleModel struct {
	ID                        types.String `tfsdk:"id"`
	RuleName                  types.String `tfsdk:"rule_name"`
	Enabled                   types.Bool   `tfsdk:"enabled"`
	Priority                  types.Int64  `tfsdk:"priority"`
	UseRegex                  types.Bool   `tfsdk:"use_regex"`
	MustContain               types.String `tfsdk:"must_contain"`
	MustNotContain            types.String `tfsdk:"must_not_contain"`
	EpisodeFilter             types.String `tfsdk:"episode_filter"`
	AffectedFeeds             types.List   `tfsdk:"affected_feeds"`
	IgnoreDays                types.Int64  `tfsdk:"ignore_days"`
	SmartFilter               types.Bool   `tfsdk:"smart_filter"`
	LastMatch                 types.String `tfsdk:"last_match"`
	PreviouslyMatchedEpisodes types.List   `tfsdk:"previously_matched_episodes"`
	AddStopped                types.Bool   `tfsdk:"add_stopped"`
	ContentLayout             types.String `tfsdk:"content_layout"`
	SavePath                  types.String `tfsdk:"save_path"`
	Category                  types.String `tfsdk:"category"`
	TorrentParams             types.Object `tfsdk:"torrent_params"`
}

func (r *rssRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rss_rule"
}

func (r *rssRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage one qBittorrent RSS auto-download rule by name. Import ID is the exact rule name. Changing the name replaces the rule. Match history is computed and carried across updates. When both a deprecated top-level alias and its equivalent torrent_params field are configured, the native torrent_params value takes precedence. Tags are an unordered set and duplicate tags are removed. Updating a rule that already contains nonempty TLS certificate, private-key, or DH-parameter material is rejected because the provider cannot preserve that unsupported material. The resource manages only the rule definition; it does not create or refresh feeds, import/export other rules, or add torrents. Keep RSS fetching and auto-downloading disabled while managing rule configuration if you do not want RSS processing.",
		Attributes: map[string]schema.Attribute{
			"id":                          schema.StringAttribute{Computed: true, MarkdownDescription: "RSS rule name.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"rule_name":                   schema.StringAttribute{Required: true, MarkdownDescription: "Exact qBittorrent rule name. Changing this replaces the resource.", Validators: []validator.String{rssRuleNameValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"enabled":                     schema.BoolAttribute{Optional: true, MarkdownDescription: "Whether qBittorrent evaluates this rule."},
			"priority":                    schema.Int64Attribute{Optional: true, MarkdownDescription: "Rule priority; qBittorrent uses lower numbered priorities first."},
			"use_regex":                   schema.BoolAttribute{Optional: true, MarkdownDescription: "Interpret must_contain and must_not_contain as regular expressions."},
			"must_contain":                schema.StringAttribute{Optional: true, MarkdownDescription: "Required title text or regular expression."},
			"must_not_contain":            schema.StringAttribute{Optional: true, MarkdownDescription: "Excluded title text or regular expression."},
			"episode_filter":              schema.StringAttribute{Optional: true, MarkdownDescription: "Episode filter expression."},
			"affected_feeds":              schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Feed URLs selected by this rule. This does not create or manage RSS feeds."},
			"ignore_days":                 schema.Int64Attribute{Optional: true, MarkdownDescription: "Ignore articles older than this number of days."},
			"smart_filter":                schema.BoolAttribute{Optional: true, MarkdownDescription: "Apply qBittorrent's smart episode filter."},
			"last_match":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Runtime last-match timestamp. Preserved on rule updates."},
			"previously_matched_episodes": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Runtime episode match history. Preserved on rule updates."},
			"add_stopped":                 schema.BoolAttribute{Optional: true, MarkdownDescription: "Deprecated qBittorrent add-paused alias. Null leaves the optional native value unset."},
			"content_layout":              schema.StringAttribute{Optional: true, MarkdownDescription: "Deprecated qBittorrent content-layout alias. Allowed values: Original, Subfolder, NoSubfolder. Null leaves the optional native value unset.", Validators: []validator.String{rssContentLayoutValidator{}}},
			"save_path":                   schema.StringAttribute{Optional: true, MarkdownDescription: "Deprecated qBittorrent save-path alias (server-side path)."},
			"category":                    schema.StringAttribute{Optional: true, MarkdownDescription: "Deprecated qBittorrent category alias."},
			"torrent_params": schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "Typed, allowlisted qBittorrent AddTorrentParams values. Secret TLS material and file-selection internals are not supported.", Attributes: map[string]schema.Attribute{
				"category": schema.StringAttribute{Optional: true}, "tags": schema.SetAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Unordered set of qBittorrent tags; duplicate values are removed."},
				"save_path": schema.StringAttribute{Optional: true}, "download_path": schema.StringAttribute{Optional: true},
				"use_download_path": schema.BoolAttribute{Optional: true}, "operating_mode": schema.StringAttribute{Optional: true, Validators: []validator.String{rssOperatingModeValidator{}}},
				"seed_mode": schema.BoolAttribute{Optional: true}, "upload_limit": schema.Int64Attribute{Optional: true}, "download_limit": schema.Int64Attribute{Optional: true},
				"ratio_limit": schema.Float64Attribute{Optional: true}, "seeding_time_limit": schema.Int64Attribute{Optional: true}, "inactive_seeding_time_limit": schema.Int64Attribute{Optional: true},
				"share_limit_action": schema.StringAttribute{Optional: true}, "share_limits_mode": schema.StringAttribute{Optional: true},
				"queue_top": schema.BoolAttribute{Optional: true}, "stopped": schema.BoolAttribute{Optional: true}, "stop_condition": schema.StringAttribute{Optional: true},
				"content_layout": schema.StringAttribute{Optional: true, Validators: []validator.String{rssContentLayoutValidator{}}}, "auto_tmm": schema.BoolAttribute{Optional: true},
			}},
		},
	}
}

func (r *rssRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	r.client, ok = req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected qBittorrent client.")
	}
}

func (r *rssRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m rssRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rules, err := r.client.RSSRules(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot check RSS rules", err.Error())
		return
	}
	name := m.RuleName.ValueString()
	if _, exists := rules[name]; exists {
		resp.Diagnostics.AddAttributeError(path.Root("rule_name"), "RSS rule already exists", "Creation never adopts or overwrites an existing rule. Import it with: terraform import <resource-address> <exact-rule-name>.")
		return
	}
	r.apply(ctx, m, nil, &resp.State, &resp.Diagnostics)
}

func (r *rssRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m rssRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rules, err := r.client.RSSRules(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read RSS rule before update", err.Error())
		return
	}
	history, exists := rules[m.RuleName.ValueString()]
	if !exists {
		resp.Diagnostics.AddError("RSS rule disappeared", "The configured rule is missing before update; refresh or import the rule again.")
		return
	}
	if history.HasUnsupportedTLSMaterial() {
		resp.Diagnostics.AddError("Cannot update RSS rule with unsupported TLS material", "This rule contains nonempty TLS certificate, private-key, or DH-parameter material that the provider cannot preserve. The update was not sent; remove or manage that material outside this resource before retrying.")
		return
	}
	r.apply(ctx, m, history, &resp.State, &resp.Diagnostics)
}

func (r *rssRuleResource) apply(ctx context.Context, m rssRuleModel, history client.RSSRule, state *tfsdk.State, d *diag.Diagnostics) {
	definition, err := rssRuleDefinition(ctx, m, history)
	if err != nil {
		d.AddError("Cannot encode RSS rule", err.Error())
		return
	}
	if err := r.client.SetRSSRule(ctx, m.RuleName.ValueString(), definition); err != nil {
		d.AddError("Cannot write RSS rule", err.Error())
		return
	}
	rules, err := r.client.RSSRules(ctx)
	if err != nil {
		d.AddError("Cannot verify RSS rule", err.Error())
		return
	}
	remote, exists := rules[m.RuleName.ValueString()]
	if !exists {
		d.AddError("RSS rule missing after update", "qBittorrent did not return the written rule.")
		return
	}
	m.ID = m.RuleName
	rssRuleRefreshConfigured(&m, remote)
	rssRuleSetHistory(ctx, &m, remote, d)
	d.Append(state.Set(ctx, &m)...)
}

func (r *rssRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.State = req.State
	var m rssRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rules, err := r.client.RSSRules(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read RSS rule", err.Error())
		return
	}
	remote, exists := rules[m.ID.ValueString()]
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}
	m.RuleName = m.ID
	rssRuleRefreshConfigured(&m, remote)
	rssRuleSetHistory(ctx, &m, remote, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *rssRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m rssRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.RemoveRSSRule(ctx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Cannot remove RSS rule", err.Error())
	}
}

func (r *rssRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError("Invalid import ID", "RSS rule import ID must be its exact nonempty qBittorrent rule name.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("rule_name"), req.ID)...)
}

func rssRuleDefinition(ctx context.Context, m rssRuleModel, history client.RSSRule) (client.RSSRule, error) {
	rule := client.RSSRule{}
	put := func(key string, value attr.Value) error {
		if value.IsNull() || value.IsUnknown() {
			return nil
		}
		v, err := rssAttrToAny(ctx, value)
		if err != nil {
			return err
		}
		rule[key] = v
		return nil
	}
	for _, pair := range []struct {
		key   string
		value attr.Value
	}{
		{"enabled", m.Enabled}, {"priority", m.Priority}, {"useRegex", m.UseRegex}, {"mustContain", m.MustContain},
		{"mustNotContain", m.MustNotContain}, {"episodeFilter", m.EpisodeFilter}, {"affectedFeeds", m.AffectedFeeds},
		{"ignoreDays", m.IgnoreDays}, {"smartFilter", m.SmartFilter}, {"addPaused", m.AddStopped},
		{"torrentContentLayout", m.ContentLayout}, {"savePath", m.SavePath}, {"assignedCategory", m.Category},
	} {
		if err := put(pair.key, pair.value); err != nil {
			return nil, err
		}
	}
	configuredParams := map[string]any{}
	if !m.TorrentParams.IsNull() && !m.TorrentParams.IsUnknown() {
		var err error
		configuredParams, err = rssObjectToMap(ctx, m.TorrentParams)
		if err != nil {
			return nil, err
		}
	}
	remoteParams, hasRemoteParams := history["torrentParams"].(map[string]any)
	if hasRemoteParams || !m.TorrentParams.IsNull() {
		params := map[string]any{}
		for key, value := range remoteParams {
			params[key] = value
		}
		for key, value := range configuredParams {
			params[key] = value
		}
		// Deprecated top-level aliases fill missing user-owned nested values.
		// A configured native value wins when both forms are present.
		if _, configured := configuredParams["category"]; !configured && !m.Category.IsNull() && !m.Category.IsUnknown() {
			params["category"] = m.Category.ValueString()
		}
		if _, configured := configuredParams["save_path"]; !configured && !m.SavePath.IsNull() && !m.SavePath.IsUnknown() {
			params["save_path"] = m.SavePath.ValueString()
		}
		if _, configured := configuredParams["stopped"]; !configured && !m.AddStopped.IsNull() && !m.AddStopped.IsUnknown() {
			params["stopped"] = m.AddStopped.ValueBool()
		}
		if _, configured := configuredParams["content_layout"]; !configured && !m.ContentLayout.IsNull() && !m.ContentLayout.IsUnknown() {
			params["content_layout"] = m.ContentLayout.ValueString()
		}
		rule["torrentParams"] = params
	}
	// Alias fields are part of qBittorrent's read schema even when null. They
	// must stay null to preserve upstream optional addStopped/torrentContentLayout.
	if m.AddStopped.IsNull() {
		rule["addPaused"] = nil
	}
	if m.ContentLayout.IsNull() {
		rule["torrentContentLayout"] = nil
	}
	if len(history) != 0 {
		rule["lastMatch"] = history["lastMatch"]
		rule["previouslyMatchedEpisodes"] = history["previouslyMatchedEpisodes"]
	}
	return rule, nil
}

func rssAttrToAny(ctx context.Context, v attr.Value) (any, error) {
	switch x := v.(type) {
	case types.String:
		return x.ValueString(), nil
	case types.Bool:
		return x.ValueBool(), nil
	case types.Int64:
		return x.ValueInt64(), nil
	case types.Float64:
		return x.ValueFloat64(), nil
	case types.List:
		var items []any
		if diags := x.ElementsAs(ctx, &items, false); !diags.HasError() {
			return items, nil
		}
		var strings []string
		if diags := x.ElementsAs(ctx, &strings, false); diags.HasError() {
			return nil, fmt.Errorf("cannot encode RSS list attribute")
		}
		return strings, nil
	case types.Set:
		var values []string
		if diags := x.ElementsAs(ctx, &values, false); diags.HasError() {
			return nil, fmt.Errorf("cannot encode RSS set attribute")
		}
		values = rssUniqueSortedStrings(values)
		items := make([]any, len(values))
		for i, value := range values {
			items[i] = value
		}
		return items, nil
	case types.Object:
		return rssObjectToMap(ctx, x)
	default:
		return nil, fmt.Errorf("unsupported RSS rule attribute type %T", v)
	}
}

func rssObjectToMap(ctx context.Context, object types.Object) (map[string]any, error) {
	values := make(map[string]any)
	for name, value := range object.Attributes() {
		if value.IsNull() || value.IsUnknown() {
			continue
		}
		mapped, err := rssAttrToAny(ctx, value)
		if err != nil {
			return nil, err
		}
		values[rssTorrentParamWireName(name)] = mapped
	}
	return values, nil
}

func rssTorrentParamWireName(terraform string) string {
	for _, pair := range [][2]string{{"queue_top", "add_to_top_of_queue"}, {"auto_tmm", "use_auto_tmm"}} {
		if terraform == pair[0] {
			return pair[1]
		}
	}
	return terraform
}

func rssRuleRefreshConfigured(m *rssRuleModel, remote client.RSSRule) {
	set := func(value *types.String, key string) {
		if !value.IsNull() {
			if v, ok := remote[key].(string); ok {
				*value = types.StringValue(v)
			}
		}
	}
	setBool := func(value *types.Bool, key string) {
		if !value.IsNull() {
			if v, ok := remote[key].(bool); ok {
				*value = types.BoolValue(v)
			}
		}
	}
	setInt := func(value *types.Int64, key string) {
		if !value.IsNull() {
			if v, ok := remote[key].(float64); ok {
				*value = types.Int64Value(int64(v))
			}
		}
	}
	set(&m.MustContain, "mustContain")
	set(&m.MustNotContain, "mustNotContain")
	set(&m.EpisodeFilter, "episodeFilter")
	setBool(&m.Enabled, "enabled")
	setBool(&m.UseRegex, "useRegex")
	setBool(&m.SmartFilter, "smartFilter")
	setInt(&m.Priority, "priority")
	setInt(&m.IgnoreDays, "ignoreDays")
	if !m.AffectedFeeds.IsNull() {
		if values, ok := remote["affectedFeeds"].([]any); ok {
			strings := make([]string, 0, len(values))
			for _, v := range values {
				if s, ok := v.(string); ok {
					strings = append(strings, s)
				}
			}
			m.AffectedFeeds, _ = types.ListValueFrom(context.Background(), types.StringType, strings)
		}
	}
	if !m.AddStopped.IsNull() && !rssNativeAliasConfigured(*m, "stopped") {
		if v, ok := remote["addPaused"].(bool); ok {
			m.AddStopped = types.BoolValue(v)
		} else {
			m.AddStopped = types.BoolNull()
		}
	}
	if !m.ContentLayout.IsNull() && !rssNativeAliasConfigured(*m, "content_layout") {
		if v, ok := remote["torrentContentLayout"].(string); ok {
			m.ContentLayout = types.StringValue(v)
		} else {
			m.ContentLayout = types.StringNull()
		}
	}
	if !rssNativeAliasConfigured(*m, "save_path") {
		set(&m.SavePath, "savePath")
	}
	if !rssNativeAliasConfigured(*m, "category") {
		set(&m.Category, "assignedCategory")
	}
	if !m.TorrentParams.IsNull() {
		if params, ok := remote["torrentParams"].(map[string]any); ok {
			rssRefreshTorrentParams(m, params)
		}
	}
}

// A configured native torrent_params value shadows its deprecated top-level
// alias. Keep that shadowed input as planned so qBittorrent's native precedence
// does not make Terraform report an inconsistent apply result.
func rssNativeAliasConfigured(m rssRuleModel, wireKey string) bool {
	if m.TorrentParams.IsNull() || m.TorrentParams.IsUnknown() {
		return false
	}
	var attribute string
	switch wireKey {
	case "category":
		attribute = "category"
	case "save_path":
		attribute = "save_path"
	case "stopped":
		attribute = "stopped"
	case "content_layout":
		attribute = "content_layout"
	default:
		return false
	}
	value, ok := m.TorrentParams.Attributes()[attribute]
	return ok && !value.IsNull() && !value.IsUnknown()
}

func rssRefreshTorrentParams(m *rssRuleModel, remote map[string]any) {
	attrs := m.TorrentParams.Attributes()
	for key, raw := range attrs {
		if raw.IsNull() || raw.IsUnknown() {
			continue
		}
		wire := rssTorrentParamWireName(key)
		value, exists := remote[wire]
		if !exists || value == nil {
			attrs[key] = rssNullAttribute(raw)
			continue
		}
		switch old := raw.(type) {
		case types.String:
			if v, ok := value.(string); ok {
				attrs[key] = types.StringValue(v)
			}
		case types.Bool:
			if v, ok := value.(bool); ok {
				attrs[key] = types.BoolValue(v)
			}
		case types.Int64:
			if v, ok := value.(float64); ok {
				attrs[key] = types.Int64Value(int64(v))
			}
		case types.Float64:
			if v, ok := value.(float64); ok {
				attrs[key] = types.Float64Value(v)
			}
		case types.List, types.Set:
			if values, ok := value.([]any); ok {
				strings := make([]string, 0, len(values))
				for _, item := range values {
					if s, ok := item.(string); ok {
						strings = append(strings, s)
					}
				}
				switch raw.(type) {
				case types.List:
					attrs[key], _ = types.ListValueFrom(context.Background(), types.StringType, strings)
				case types.Set:
					strings = rssUniqueSortedStrings(strings)
					elements := make([]attr.Value, len(strings))
					for i, value := range strings {
						elements[i] = types.StringValue(value)
					}
					attrs[key], _ = types.SetValue(types.StringType, elements)
				}
			}
		default:
			_ = old
		}
	}
	updated, err := types.ObjectValue(m.TorrentParams.AttributeTypes(context.Background()), attrs)
	if err == nil {
		m.TorrentParams = updated
	}
}

func rssUniqueSortedStrings(values []string) []string {
	sort.Strings(values)
	unique := values[:0]
	for _, value := range values {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	return unique
}

func rssNullAttribute(value attr.Value) attr.Value {
	switch value.(type) {
	case types.String:
		return types.StringNull()
	case types.Bool:
		return types.BoolNull()
	case types.Int64:
		return types.Int64Null()
	case types.Float64:
		return types.Float64Null()
	case types.List:
		return types.ListNull(types.StringType)
	case types.Set:
		return types.SetNull(types.StringType)
	default:
		return value
	}
}

func rssRuleSetHistory(ctx context.Context, m *rssRuleModel, remote client.RSSRule, d *diag.Diagnostics) {
	if value, ok := remote["lastMatch"].(string); ok {
		m.LastMatch = types.StringValue(value)
	} else {
		m.LastMatch = types.StringValue("")
	}
	items := []string{}
	if values, ok := remote["previouslyMatchedEpisodes"].([]any); ok {
		for _, item := range values {
			if s, ok := item.(string); ok {
				items = append(items, s)
			}
		}
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, items)
	d.Append(diags...)
	m.PreviouslyMatchedEpisodes = list
}

type rssContentLayoutValidator struct{}

func (rssContentLayoutValidator) Description(context.Context) string {
	return "must be Original, Subfolder, or NoSubfolder"
}
func (v rssContentLayoutValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (rssContentLayoutValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	switch req.ConfigValue.ValueString() {
	case "Original", "Subfolder", "NoSubfolder":
	default:
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid content layout", "Use Original, Subfolder, or NoSubfolder.")
	}
}

type rssOperatingModeValidator struct{}

func (rssOperatingModeValidator) Description(context.Context) string {
	return "must be AutoManaged or Forced"
}

type rssRuleNameValidator struct{}

func (rssRuleNameValidator) Description(context.Context) string { return "must not be empty" }
func (v rssRuleNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (rssRuleNameValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && req.ConfigValue.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid RSS rule name", "The qBittorrent rule name must not be empty.")
	}
}
func (v rssOperatingModeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (rssOperatingModeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	switch req.ConfigValue.ValueString() {
	case "AutoManaged", "Forced":
	default:
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid operating mode", "Use AutoManaged or Forced.")
	}
}

func rssRuleTorrentParamAttributeTypes(ctx context.Context) map[string]attr.Type {
	var resp resource.SchemaResponse
	(&rssRuleResource{}).Schema(ctx, resource.SchemaRequest{}, &resp)
	nested := resp.Schema.Attributes["torrent_params"].(schema.SingleNestedAttribute)
	attrs := map[string]attr.Type{}
	for key, attribute := range nested.Attributes {
		attrs[key] = attribute.GetType()
	}
	return attrs
}

var _ resource.Resource = (*rssRuleResource)(nil)
var _ resource.ResourceWithImportState = (*rssRuleResource)(nil)
