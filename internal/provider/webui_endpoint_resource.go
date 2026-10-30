package provider

import (
	"context"
	"fmt"
	"net"

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

type webuiEndpointResource struct{ client *client.Client }

func (r *webuiEndpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webui_endpoint"
}
func (r *webuiEndpointResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Explicit maintenance of the qBittorrent WebUI listener. A changed listener requires post_update_url and uses a separate candidate connection to verify the result. If verification fails, update the provider URL manually and reapply only after checking the server; the mutation is never replayed. Import ID: application. Apply this separately from ordinary preferences and update the provider endpoint after a successful transition.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"web_ui_address":         schema.StringAttribute{Optional: true, Computed: true, Validators: []validator.String{webUIBindValidator{}}, MarkdownDescription: "Server bind address: * or an IPv4/IPv6 address."},
			"web_ui_port":            schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "WebUI TCP port, 1 through 65535."},
			"use_https":              schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether the WebUI uses HTTPS. TLS verification remains enabled for candidate connections."},
			"web_ui_https_cert_path": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Server-side certificate path; certificate contents are never managed."},
			"web_ui_https_key_path":  schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Server-side private key path; key contents are never managed."},
			"post_update_url":        schema.StringAttribute{Optional: true, MarkdownDescription: "Target WebUI URL used only for the bounded post-update reconnect. Required whenever any listener field changes; set the provider URL to this value in the next maintenance operation."},
		},
	}
}
func (r *webuiEndpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected qBittorrent client.")
		return
	}
	r.client = c
}
func (r *webuiEndpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var cfg, plan types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := endpointPatch(cfg, nil, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
	r.apply(ctx, patch, cfg, plan, &resp.State, &resp.Diagnostics)
}
func (r *webuiEndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var cfg, plan, prior types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := endpointPatch(cfg, &prior, false, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
	r.apply(ctx, patch, cfg, plan, &resp.State, &resp.Diagnostics)
}
func endpointPatch(cfg types.Object, prior *types.Object, creating bool, d *diag.Diagnostics) map[string]any {
	patch := map[string]any{}
	for name, wire := range map[string]string{"web_ui_address": "web_ui_address", "web_ui_port": "web_ui_port", "use_https": "use_https", "web_ui_https_cert_path": "web_ui_https_cert_path", "web_ui_https_key_path": "web_ui_https_key_path"} {
		v := cfg.Attributes()[name]
		if isUnknownValue(v) {
			d.AddAttributeError(path.Root(name), "Unknown WebUI endpoint value", "Endpoint values must be known before the listener can be updated.")
			return nil
		}
		if isNullValue(v) {
			continue
		}
		old := any(nil)
		if !creating && prior != nil {
			old = prior.Attributes()[name]
		}
		if !creating && equalFrameworkValue(v, old) {
			continue
		}
		switch x := v.(type) {
		case types.String:
			patch[wire] = x.ValueString()
		case types.Int64:
			if x.ValueInt64() < 1 || x.ValueInt64() > 65535 {
				d.AddAttributeError(path.Root(name), "Invalid WebUI port", "web_ui_port must be between 1 and 65535.")
				return nil
			}
			patch[wire] = x.ValueInt64()
		case types.Bool:
			patch[wire] = x.ValueBool()
		}
	}
	if len(patch) > 0 {
		target, ok := cfg.Attributes()["post_update_url"].(types.String)
		if !ok || target.IsNull() || target.IsUnknown() || target.ValueString() == "" {
			d.AddAttributeError(path.Root("post_update_url"), "Target URL required", "Set post_update_url whenever a WebUI listener field changes.")
			return nil
		}
		if _, err := client.New(client.Config{URL: target.ValueString(), Username: "candidate", Password: "candidate"}); err != nil { // URL syntax validation without depending on the current auth mode.
			d.AddAttributeError(path.Root("post_update_url"), "Invalid target URL", "post_update_url must be an absolute HTTP(S) WebUI URL without credentials, query or fragment.")
			return nil
		}
	}
	return patch
}
func (r *webuiEndpointResource) apply(ctx context.Context, patch map[string]any, cfg, plan types.Object, state *tfsdk.State, d *diag.Diagnostics) {
	if len(patch) == 0 {
		if p, err := r.client.Preferences(ctx); err == nil {
			setEndpointState(ctx, state, p, d)
			setConfiguredEndpointFields(ctx, state, cfg, d)
		}
		return
	}
	target := plan.Attributes()["post_update_url"].(types.String).ValueString()
	current, err := r.client.Preferences(ctx)
	if err != nil {
		d.AddError("Cannot inspect WebUI endpoint", "The current WebUI preferences could not be read, so no listener update was sent.")
		return
	}
	for key := range patch {
		if !webUIPreferenceHasField(current, key) {
			d.AddAttributeError(path.Root(key), "Unsupported WebUI endpoint field", "The server did not return this established WebUI preference; no listener update was sent.")
			return
		}
	}
	setEndpointState(ctx, state, current, d)
	setConfiguredEndpointFields(ctx, state, cfg, d)
	remaining := make(map[string]any, len(patch))
	for key, wanted := range patch {
		if !webUIPreferenceMatches(current, key, wanted) {
			remaining[key] = wanted
		}
	}
	var writeErr error
	if len(remaining) > 0 {
		writeErr = r.client.SetWebUIMaintenancePatch(ctx, remaining, false) // exactly one mutation attempt
	}
	candidate, err := r.client.MaintenanceCandidate(target, "", "")
	if err == nil {
		err = candidate.VerifyWebUIEndpoint(ctx, patch)
	}
	if err != nil {
		outcome := "The listener mutation outcome is uncertain."
		if len(remaining) == 0 || writeErr == nil {
			outcome = "The listener mutation was accepted but reconnect verification failed."
		}
		d.AddWarning("WebUI endpoint reconnect is unverified", fmt.Sprintf("%s The resource state is retained; refresh from the reachable target endpoint before planning another write. No mutation will be replayed automatically.", outcome))
	} else if targetPreferences, readErr := candidate.Preferences(ctx); readErr == nil {
		setEndpointState(ctx, state, targetPreferences, d)
		setConfiguredEndpointFields(ctx, state, cfg, d)
	}
}

func setEndpointState(ctx context.Context, state *tfsdk.State, p client.Preferences, d *diag.Diagnostics) {
	d.Append(state.SetAttribute(ctx, path.Root("web_ui_address"), stringPtrValue(p.WebUIAddress))...)
	d.Append(state.SetAttribute(ctx, path.Root("web_ui_port"), int32PtrValue(p.WebUIPort))...)
	d.Append(state.SetAttribute(ctx, path.Root("use_https"), boolPtrValue(p.UseHttps))...)
	d.Append(state.SetAttribute(ctx, path.Root("web_ui_https_cert_path"), stringPtrValue(p.WebUIHttpsCertPath))...)
	d.Append(state.SetAttribute(ctx, path.Root("web_ui_https_key_path"), stringPtrValue(p.WebUIHttpsKeyPath))...)
}

func setConfiguredEndpointFields(ctx context.Context, state *tfsdk.State, cfg types.Object, d *diag.Diagnostics) {
	for name, value := range cfg.Attributes() {
		if isNullValue(value) {
			continue
		}
		switch x := value.(type) {
		case types.String:
			d.Append(state.SetAttribute(ctx, path.Root(name), x)...)
		case types.Int64:
			d.Append(state.SetAttribute(ctx, path.Root(name), x)...)
		case types.Bool:
			d.Append(state.SetAttribute(ctx, path.Root(name), x)...)
		}
	}
}

func webUIPreferenceMatches(p client.Preferences, key string, wanted any) bool {
	switch key {
	case "web_ui_address":
		return p.WebUIAddress != nil && *p.WebUIAddress == wanted.(string)
	case "web_ui_port":
		return p.WebUIPort != nil && int64(*p.WebUIPort) == wanted.(int64)
	case "use_https":
		return p.UseHttps != nil && *p.UseHttps == wanted.(bool)
	case "web_ui_https_cert_path":
		return p.WebUIHttpsCertPath != nil && *p.WebUIHttpsCertPath == wanted.(string)
	case "web_ui_https_key_path":
		return p.WebUIHttpsKeyPath != nil && *p.WebUIHttpsKeyPath == wanted.(string)
	default:
		return false
	}
}

func webUIPreferenceHasField(p client.Preferences, key string) bool {
	switch key {
	case "web_ui_address":
		return p.WebUIAddress != nil
	case "web_ui_port":
		return p.WebUIPort != nil
	case "use_https":
		return p.UseHttps != nil
	case "web_ui_https_cert_path":
		return p.WebUIHttpsCertPath != nil
	case "web_ui_https_key_path":
		return p.WebUIHttpsKeyPath != nil
	default:
		return false
	}
}
func isNullValue(v any) bool {
	switch x := v.(type) {
	case types.String:
		return x.IsNull() || x.IsUnknown()
	case types.Int64:
		return x.IsNull() || x.IsUnknown()
	case types.Bool:
		return x.IsNull() || x.IsUnknown()
	}
	return true
}
func isUnknownValue(v any) bool {
	switch x := v.(type) {
	case types.String:
		return x.IsUnknown()
	case types.Int64:
		return x.IsUnknown()
	case types.Bool:
		return x.IsUnknown()
	}
	return false
}
func equalFrameworkValue(a, b any) bool {
	switch x := a.(type) {
	case types.String:
		y, ok := b.(types.String)
		return ok && x.Equal(y)
	case types.Int64:
		y, ok := b.(types.Int64)
		return ok && x.Equal(y)
	case types.Bool:
		y, ok := b.(types.Bool)
		return ok && x.Equal(y)
	}
	return false
}
func (r *webuiEndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.State = req.State
	p, err := r.client.Preferences(ctx)
	if err != nil {
		resp.Diagnostics.AddWarning("WebUI endpoint refresh unavailable", "The configured provider endpoint cannot reach qBittorrent. State was retained; set the provider URL to the resource's post_update_url and refresh again. No listener write was sent.")
		return
	}
	for _, key := range []string{"web_ui_address", "web_ui_port", "use_https", "web_ui_https_cert_path", "web_ui_https_key_path"} {
		if !webUIPreferenceHasField(p, key) {
			resp.Diagnostics.AddError("WebUI endpoint field unavailable", "The server omitted an established WebUI endpoint preference; state was retained and no listener write was sent.")
			return
		}
	}
	setEndpointState(ctx, &resp.State, p, &resp.Diagnostics)
}

func stringPtrValue(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}
func int32PtrValue(v *int32) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*v))
}
func boolPtrValue(v *bool) types.Bool {
	if v == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*v)
}
func (r *webuiEndpointResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
func (r *webuiEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "application" {
		resp.Diagnostics.AddError("Invalid import ID", "WebUI endpoint import ID must be application.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

var _ resource.Resource = (*webuiEndpointResource)(nil)
var _ resource.ResourceWithImportState = (*webuiEndpointResource)(nil)

type webUIBindValidator struct{}

func (webUIBindValidator) Description(context.Context) string {
	return "must be * or an IPv4/IPv6 address"
}
func (webUIBindValidator) MarkdownDescription(ctx context.Context) string {
	return webUIBindValidator{}.Description(ctx)
}
func (webUIBindValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v := req.ConfigValue.ValueString()
	if v != "*" && net.ParseIP(v) == nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid WebUI bind address", "Use * or a valid IPv4/IPv6 address.")
	}
}
