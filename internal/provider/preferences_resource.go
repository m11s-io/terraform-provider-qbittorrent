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
			if f.readOnly {
				attrs[f.name] = schema.StringAttribute{Computed: true, MarkdownDescription: f.description}
			} else {
				validators := webUIStringValidators(f.name, r.webui)
				if len(f.stringEnums) > 0 {
					validators = append(validators, stringChoiceValidator{allowed: f.stringEnums})
				}
				attrs[f.name] = schema.StringAttribute{Optional: true, Sensitive: f.sensitive, MarkdownDescription: f.description, Validators: validators}
			}
		case "bool":
			if f.readOnly {
				attrs[f.name] = schema.BoolAttribute{Computed: true, MarkdownDescription: f.description}
			} else {
				attrs[f.name] = schema.BoolAttribute{Optional: true, Sensitive: f.sensitive, MarkdownDescription: f.description}
			}
		case "int":
			if f.readOnly {
				attrs[f.name] = schema.Int64Attribute{Computed: true, MarkdownDescription: f.description}
			} else {
				attrs[f.name] = schema.Int64Attribute{Optional: true, Sensitive: f.sensitive, MarkdownDescription: preferenceResourceDescription(f), Validators: []validator.Int64{preferenceIntValidator{field: f}}}
			}
		case "float":
			if f.readOnly {
				attrs[f.name] = schema.Float64Attribute{Computed: true, MarkdownDescription: f.description}
			} else {
				attrs[f.name] = schema.Float64Attribute{Optional: true, Sensitive: f.sensitive, MarkdownDescription: f.description, Validators: []validator.Float64{preferenceFloatValidator{field: f}}}
			}
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
	capabilityFields := configuredCapabilityFields(r.fields(), obj.Attributes())
	if len(capabilityFields) > 0 {
		response, err := r.readPreferences(ctx)
		if err != nil {
			d.AddError("Cannot verify preference support", "The server capability could not be checked before writing.")
			return
		}
		b, err := json.Marshal(response)
		if err != nil {
			d.AddError("Cannot verify preference support", "The server capability response could not be checked before writing.")
			return
		}
		var remote map[string]json.RawMessage
		if err := json.Unmarshal(b, &remote); err != nil {
			d.AddError("Cannot verify preference support", "The server capability response could not be checked before writing.")
			return
		}
		for _, f := range unsupportedCapabilityFields(capabilityFields, remote) {
			d.AddAttributeError(path.Root(f.name), "Unsupported preference capability", fmt.Sprintf("The server response does not expose this preference (required capability %s). No update was sent.", f.capability))
		}
		if d.HasError() {
			return
		}
	}
	patch := map[string]any{}
	for k, v := range obj.Attributes() {
		if k == "id" || v.IsNull() || fieldReadOnly(r.fields(), k) {
			continue
		}
		if v.IsUnknown() {
			d.AddAttributeError(path.Root(k), "Unknown preference", "Managed preferences must be known at apply.")
			continue
		}
		switch v := v.(type) {
		case types.String:
			if wire, value, include := preferencePatchEntry(r.fields(), k, v.ValueString()); include {
				patch[wire] = value
			}
		case types.Bool:
			if wire, value, include := preferencePatchEntry(r.fields(), k, v.ValueBool()); include {
				patch[wire] = value
			}
		case types.Int64:
			if wire, value, include := preferencePatchEntry(r.fields(), k, v.ValueInt64()); include {
				patch[wire] = value
			}
		case types.Float64:
			if wire, value, include := preferencePatchEntry(r.fields(), k, v.ValueFloat64()); include {
				patch[wire] = value
			}
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
			var p client.PreferencesPatch
			if err = json.Unmarshal(b, &p); err != nil {
				d.AddError("Cannot encode preferences", "Preference types do not match the client contract.")
				return
			}
			if err = r.client.SetPreferencesPatch(ctx, p); err != nil {
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
		raw, ok := remote[fieldWireKey(r.fields(), k)]
		if !ok {
			if f, exists := findPreferenceField(r.fields(), k); exists && f.readOnly && f.capability != "" {
				values[k] = nullPreferenceValue(v)
				continue
			}
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

func nullPreferenceValue(v attr.Value) attr.Value {
	switch v.(type) {
	case types.String:
		return types.StringNull()
	case types.Bool:
		return types.BoolNull()
	case types.Int64:
		return types.Int64Null()
	case types.Float64:
		return types.Float64Null()
	default:
		return v
	}
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
	case types.Float64:
		var v float64
		err := json.Unmarshal(b, &v)
		return types.Float64Value(v), err
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
