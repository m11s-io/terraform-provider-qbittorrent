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
			"web_ui_address":         schema.StringAttribute{Optional: true, Validators: []validator.String{webUIBindValidator{}}, MarkdownDescription: "Server bind address: * or an IPv4/IPv6 address."},
			"web_ui_port":            schema.Int64Attribute{Optional: true, MarkdownDescription: "WebUI TCP port, 1 through 65535."},
			"use_https":              schema.BoolAttribute{Optional: true, MarkdownDescription: "Whether the WebUI uses HTTPS. TLS verification remains enabled for candidate connections."},
			"web_ui_https_cert_path": schema.StringAttribute{Optional: true, MarkdownDescription: "Server-side certificate path; certificate contents are never managed."},
			"web_ui_https_key_path":  schema.StringAttribute{Optional: true, MarkdownDescription: "Server-side private key path; key contents are never managed."},
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
	attempted := false
	if !resp.Diagnostics.HasError() {
		attempted = len(patch) > 0
		r.apply(ctx, patch, plan, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		if attempted {
			resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
		}
		return
	}
	resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
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
	attempted := false
	if !resp.Diagnostics.HasError() {
		attempted = len(patch) > 0
		r.apply(ctx, patch, plan, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		if attempted {
			resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
		}
		return
	}
	resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
}
func endpointPatch(cfg types.Object, prior *types.Object, creating bool, d *diag.Diagnostics) map[string]any {
	patch := map[string]any{}
	for name, wire := range map[string]string{"web_ui_address": "web_ui_address", "web_ui_port": "web_ui_port", "use_https": "use_https", "web_ui_https_cert_path": "web_ui_https_cert_path", "web_ui_https_key_path": "web_ui_https_key_path"} {
		v := cfg.Attributes()[name]
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
func (r *webuiEndpointResource) apply(ctx context.Context, patch map[string]any, plan types.Object, d *diag.Diagnostics) {
	if len(patch) == 0 {
		return
	}
	target := plan.Attributes()["post_update_url"].(types.String).ValueString()
	writeErr := r.client.SetWebUIMaintenancePatch(ctx, patch, false) // exactly one mutation attempt
	candidate, err := r.client.MaintenanceCandidate(target, "", "")
	if err == nil {
		err = candidate.VerifyWebUIEndpoint(ctx, patch)
	}
	if err != nil {
		outcome := "The listener mutation outcome is uncertain."
		if writeErr == nil {
			outcome = "The listener mutation was accepted but reconnect verification failed."
		}
		d.AddError("WebUI endpoint reconnect failed", fmt.Sprintf("%s Set the provider URL to the configured post_update_url, verify the WebUI manually, then run a targeted maintenance apply. No mutation was replayed.", outcome))
		return
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
	var state types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	expected := map[string]any{}
	for name, wire := range map[string]string{"web_ui_address": "web_ui_address", "web_ui_port": "web_ui_port", "use_https": "use_https", "web_ui_https_cert_path": "web_ui_https_cert_path", "web_ui_https_key_path": "web_ui_https_key_path"} {
		v := state.Attributes()[name]
		if isNullValue(v) {
			continue
		}
		switch x := v.(type) {
		case types.String:
			expected[wire] = x.ValueString()
		case types.Int64:
			expected[wire] = x.ValueInt64()
		case types.Bool:
			expected[wire] = x.ValueBool()
		}
	}
	if len(expected) > 0 {
		if err := r.client.VerifyWebUIEndpoint(ctx, expected); err != nil {
			resp.Diagnostics.AddError("WebUI endpoint state is unverified", "Connect the provider to the configured post_update_url, inspect the WebUI listener, and run a targeted maintenance refresh. State was retained and no write was sent.")
		}
	}
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
