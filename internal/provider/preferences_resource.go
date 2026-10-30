package provider

import (
	"context"
	"encoding/json"
	"fmt"

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

type preferencesResource struct {
	client *client.Client
	webui  bool
}

type preferenceField struct{ name, kind, description string }

// The field table defines only public nonsecret attributes. All optional values
// remain nullable: null relinquishes ownership, false/zero are managed values.
var preferenceFields = []preferenceField{
	{"locale", "string", "Behavior: user interface language (upstream locale identifier)."},
	{"confirm_torrent_deletion", "bool", "Behavior: confirm torrent deletion."},
	{"status_bar_external_ip", "bool", "Behavior: show external IP in the status bar."},
	{"store_search_jobs", "bool", "Behavior: save search tabs."},
	{"store_search_job_results", "bool", "Behavior: also save search results."},
	{"performance_warning", "bool", "Behavior: log performance warnings."},
	{"file_log_enabled", "bool", "Behavior: enable file logging."},
	{"file_log_path", "string", "Behavior: log directory on the server."},
	{"file_log_backup_enabled", "bool", "Behavior: rotate log files by size."},
	{"file_log_max_size", "log_size", "Behavior: rotate logs after this size in KiB, 1..1024000."},
	{"file_log_delete_old", "bool", "Behavior: delete old backup logs."},
	{"file_log_age", "log_age", "Behavior: backup log retention count, 1..365."},
	{"file_log_age_type", "log_unit", "Behavior: retention unit, 0 days, 1 months, 2 years."},

	{"save_path", "string", "Default save path on the server. Use a normalized absolute path."},
	{"temp_path", "string", "Incomplete-download path on the server. Use a normalized absolute path."},
	{"temp_path_enabled", "bool", "Enable the incomplete-download path."},
	{"auto_tmm_enabled", "bool", "Enable automatic torrent management by default. Changes may relocate files."},
	{"dl_limit", "rate", "Global download limit in bytes/s; 0 means unlimited."},
	{"up_limit", "rate", "Global upload limit in bytes/s; 0 means unlimited."},
	{"alt_dl_limit", "rate", "Alternate download limit in bytes/s; 0 means unlimited."},
	{"alt_up_limit", "rate", "Alternate upload limit in bytes/s; 0 means unlimited."},
	{"scheduler_enabled", "bool", "Schedule the use of alternative rate limits using the server's local time."},
	{"schedule_from_hour", "hour", "Schedule start hour, 0..23. Configure together with schedule_from_min."},
	{"schedule_from_min", "minute", "Schedule start minute, 0..59. Configure together with schedule_from_hour."},
	{"schedule_to_hour", "hour", "Schedule end hour, 0..23. Configure together with schedule_to_min."},
	{"schedule_to_min", "minute", "Schedule end minute, 0..59. Configure together with schedule_to_hour."},
	{"scheduler_days", "days", "Schedule days: 0 every day, 1 weekdays, 2 weekends, 3 Monday, 4 Tuesday, 5 Wednesday, 6 Thursday, 7 Friday, 8 Saturday, 9 Sunday."},
	{"limit_utp_rate", "bool", "Apply rate limits to the uTP protocol."},
	{"limit_tcp_overhead", "bool", "Apply rate limits to transport overhead."},
	{"limit_lan_peers", "bool", "Apply rate limits to peers on the LAN."},
	{"dht", "bool", "Enable DHT."}, {"pex", "bool", "Enable peer exchange."}, {"lsd", "bool", "Enable local service discovery."},
	{"listen_port", "port", "Incoming listen port, 1..65535. Disable random-port settings on the server before managing this field."},
	{"upnp", "bool", "Enable UPnP/NAT-PMP port forwarding."},
}

func (r *preferencesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_preferences"
	if r.webui {
		resp.TypeName = req.ProviderTypeName + "_webui_preferences"
	}
}
func (r *preferencesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Singleton ID: application.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}}
	for _, f := range r.fields() {
		switch f.kind {
		case "string":
			attrs[f.name] = schema.StringAttribute{Optional: true, MarkdownDescription: f.description, Validators: webUIStringValidators(f.name, r.webui)}
		case "bool":
			attrs[f.name] = schema.BoolAttribute{Optional: true, MarkdownDescription: f.description}
		case "hour", "minute", "days", "log_size", "log_age", "log_unit":
			max := map[string]int64{"hour": 23, "minute": 59, "days": 9, "log_size": 1024000, "log_age": 365, "log_unit": 2}[f.kind]
			min := int64(0)
			if f.kind == "log_size" || f.kind == "log_age" {
				min = 1
			}
			attrs[f.name] = schema.Int64Attribute{Optional: true, MarkdownDescription: f.description, Validators: []validator.Int64{rangeValidator{min: min, max: max}}}
		default:
			description := f.description
			if f.kind == "rate" {
				description += " Rate limits must be multiples of 1024, at most 2147482624."
			}
			attrs[f.name] = schema.Int64Attribute{Optional: true, MarkdownDescription: description, Validators: []validator.Int64{numberValidator{port: f.kind == "port"}}}
		}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Singleton partial ownership of application preferences. Configure only settings to manage. Removing an attribute relinquishes management without resetting it. Import ID: application; import reads the server without claiming unconfigured settings. Destroy removes state only. Use one instance per server; overlapping ownership is unsupported. Read-after-write reports normalized/rejected fields: use server-normalized paths.", Attributes: attrs}
	if r.webui {
		resp.Schema.MarkdownDescription = "Singleton partial ownership of server-stored WebUI Behavior settings. Import ID: application. Unset attributes remain unmanaged; removing an attribute or destroying the resource preserves stored settings. color_scheme=auto and display_density=default explicitly reset their overrides. Changes may require a browser reload. Use one instance per server."
	}
}
func (r *preferencesResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var obj types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &obj)...)
	if resp.Diagnostics.HasError() || obj.IsNull() || obj.IsUnknown() {
		return
	}
	validateSchedulePairs(obj.Attributes(), &resp.Diagnostics)
}

// qBittorrent ignores an hour or minute update unless both keys are present.
func validateSchedulePairs(values map[string]attr.Value, d *diag.Diagnostics) {
	for _, prefix := range []string{"schedule_from_", "schedule_to_"} {
		h, m := values[prefix+"hour"], values[prefix+"min"]
		if h == nil || m == nil || h.IsUnknown() || m.IsUnknown() {
			continue
		}
		if h.IsNull() != m.IsNull() {
			d.AddAttributeError(path.Root(prefix+"hour"), "Incomplete schedule time", "Configure both "+prefix+"hour and "+prefix+"min, or leave both unset.")
		}
	}
}
func (r *preferencesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	r.client, ok = req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected qBittorrent client.")
	}
}
func (r *preferencesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	planned := tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	r.apply(ctx, planned, &resp.State, &resp.Diagnostics)
}
func (r *preferencesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	planned := tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	r.apply(ctx, planned, &resp.State, &resp.Diagnostics)
}
func (r *preferencesResource) apply(ctx context.Context, planned tfsdk.State, state *tfsdk.State, d *diag.Diagnostics) {
	var obj types.Object
	d.Append(planned.Get(ctx, &obj)...)
	if d.HasError() {
		return
	}
	validateSchedulePairs(obj.Attributes(), d)
	if d.HasError() {
		return
	}
	patch := map[string]any{}
	for k, v := range obj.Attributes() {
		if k == "id" || v.IsNull() {
			continue
		}
		if v.IsUnknown() {
			d.AddAttributeError(path.Root(k), "Unknown preference", "Managed preferences must be known at apply.")
			continue
		}
		switch v := v.(type) {
		case types.String:
			patch[k] = v.ValueString()
		case types.Bool:
			patch[k] = v.ValueBool()
		case types.Int64:
			patch[k] = v.ValueInt64()
		}
	}
	if d.HasError() {
		return
	}
	if len(patch) > 0 {
		b, err := json.Marshal(patch)
		if err != nil {
			d.AddError("Cannot encode preferences", err.Error())
			return
		}
		if r.webui {
			if err = r.client.SetWebUIPreferences(ctx, patch); err != nil {
				d.AddError("Cannot update WebUI preferences", err.Error())
				return
			}
		} else {
			var p client.Preferences
			if err = json.Unmarshal(b, &p); err != nil {
				d.AddError("Cannot encode preferences", "Preference types do not match the client contract.")
				return
			}
			if err = r.client.SetPreferences(ctx, p); err != nil {
				d.AddError("Cannot update preferences", err.Error())
				return
			}
		}
	}
	// Preserve the framework-provided null/prior response state on rejected writes.
	*state = planned
	d.Append(state.SetAttribute(ctx, path.Root("id"), "application")...)
	r.refresh(ctx, state, d, true)
}
func (r *preferencesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.State = req.State
	r.refresh(ctx, &resp.State, &resp.Diagnostics, false)
}
func (r *preferencesResource) refresh(ctx context.Context, state *tfsdk.State, d *diag.Diagnostics, compare bool) {
	p, err := r.readPreferences(ctx)
	if err != nil {
		d.AddError("Cannot read preferences", err.Error())
		return
	}
	var obj types.Object
	d.Append(state.Get(ctx, &obj)...)
	if d.HasError() {
		return
	}
	values := obj.Attributes()
	b, _ := json.Marshal(p)
	var remote map[string]json.RawMessage
	json.Unmarshal(b, &remote)
	for k, v := range values {
		if k == "id" || v.IsNull() {
			continue
		}
		raw, ok := remote[k]
		if !ok {
			d.AddError("Unsupported preference response", fmt.Sprintf("Server omitted managed preference %s.", k))
			continue
		}
		actual, err := preferenceValue(v, raw)
		if err != nil {
			d.AddError("Invalid preference response", fmt.Sprintf("Server returned invalid preference %s.", k))
			continue
		}
		values[k] = actual
		if compare && !v.Equal(actual) {
			d.AddAttributeError(path.Root(k), "Preference rejected or normalized", "The server returned a different value for this field. Use the server-normalized path/value; check conflicting server settings.")
		}
	}
	value, ds := types.ObjectValue(obj.AttributeTypes(ctx), values)
	d.Append(ds...)
	d.Append(state.Set(ctx, value)...)
}
func preferenceValue(kind attr.Value, b []byte) (attr.Value, error) {
	switch kind.(type) {
	case types.String:
		var v string
		err := json.Unmarshal(b, &v)
		return types.StringValue(v), err
	case types.Bool:
		var v bool
		err := json.Unmarshal(b, &v)
		return types.BoolValue(v), err
	case types.Int64:
		var v int64
		err := json.Unmarshal(b, &v)
		return types.Int64Value(v), err
	}
	return nil, fmt.Errorf("unsupported preference type")
}
func (r *preferencesResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
func (r *preferencesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "application" {
		resp.Diagnostics.AddError("Invalid import ID", "Use application to import preferences.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
