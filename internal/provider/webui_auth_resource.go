package provider

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf16"

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

type webuiAuthResource struct{ client *client.Client }

var webUISecret = versionedSecretSpec{valueAttribute: "password_wo", versionAttribute: "password_wo_version", wireKey: "web_ui_password", description: "WebUI password (write-only); qBittorrent requires at least six characters."}

func (r *webuiAuthResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webui_auth"
}
func (r *webuiAuthResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Explicit WebUI username/password maintenance. Password is write-only and sent only when password_wo_version changes. qBittorrent requires username length at least three with no colon, and password length at least six. Bearer API-key authentication cannot change its own WebUI username/password. Import ID: application. Apply separately, then configure the provider with the new credentials.", Attributes: map[string]schema.Attribute{
		"id":                  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"username":            schema.StringAttribute{Required: true, Validators: []validator.String{webUIUsernameValidator{}}},
		"password_wo":         schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, Validators: []validator.String{webUIPasswordValidator{}}, MarkdownDescription: "New WebUI password. Write-only; set with password_wo_version."},
		"password_wo_version": schema.Int64Attribute{Optional: true, Validators: []validator.Int64{nonNegativeVersionValidator{}}},
	}}
}
func (r *webuiAuthResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *webuiAuthResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var cfg, plan types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := authPatch(cfg, nil, plan, true, &resp.Diagnostics)
	attempted := false
	if !resp.Diagnostics.HasError() {
		attempted = r.apply(ctx, patch, cfg, plan, &resp.Diagnostics)
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
func (r *webuiAuthResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var cfg, plan, prior types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := authPatch(cfg, &prior, plan, false, &resp.Diagnostics)
	attempted := false
	if !resp.Diagnostics.HasError() {
		attempted = r.apply(ctx, patch, cfg, plan, &resp.Diagnostics)
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
func authPatch(cfg types.Object, prior *types.Object, plan types.Object, creating bool, d *diag.Diagnostics) map[string]any {
	patch := map[string]any{}
	username := cfg.Attributes()["username"].(types.String)
	if username.IsUnknown() {
		d.AddAttributeError(path.Root("username"), "Unknown WebUI username", "The target username must be known before an authentication update.")
		return nil
	}
	if creating || prior == nil || !username.Equal(prior.Attributes()["username"].(types.String)) {
		patch["web_ui_username"] = username.ValueString()
	}
	var old types.Object
	if prior != nil {
		old = *prior
	}
	wire, value, include := versionedSecretPatch(cfg, plan, old, webUISecret, creating, d)
	if include {
		patch[wire] = value
	}
	if len(patch) > 0 {
		pw, ok := cfg.Attributes()["password_wo"].(types.String)
		if !ok || pw.IsNull() || pw.IsUnknown() {
			d.AddAttributeError(path.Root("password_wo"), "Password required for authentication transition", "Configure password_wo and its version to reconnect with the target WebUI credentials.")
			return nil
		}
		if pw.ValueString() == "" || nativeQStringLength(pw.ValueString()) < 6 {
			d.AddAttributeError(path.Root("password_wo"), "Invalid WebUI password", "qBittorrent requires a password of at least six characters.")
			return nil
		}
	}
	return patch
}
func (r *webuiAuthResource) apply(ctx context.Context, patch map[string]any, cfg, plan types.Object, d *diag.Diagnostics) bool {
	if len(patch) == 0 {
		return true
	}
	if r.client.UsesBearerAuthentication() {
		d.AddError("Unsupported WebUI authentication transition", "The active provider uses Bearer API-key authentication and cannot update its own username/password. Reconfigure the provider with session username/password first.")
		return false
	}
	username := cfg.Attributes()["username"].(types.String).ValueString()
	password := cfg.Attributes()["password_wo"].(types.String).ValueString()
	writeErr := r.client.SetWebUIMaintenancePatch(ctx, patch, true) // one mutation attempt only
	candidate, err := r.client.MaintenanceCandidate("", username, password)
	if err == nil {
		var current client.Preferences
		current, err = candidate.Preferences(ctx)
		if err == nil && current.WebUIUsername == nil {
			err = fmt.Errorf("qBittorrent did not return the WebUI username")
		}
		if err == nil && *current.WebUIUsername != username {
			err = fmt.Errorf("qBittorrent did not apply the target WebUI username")
		}
	}
	if err != nil {
		outcome := "The authentication mutation outcome is uncertain."
		if writeErr == nil {
			outcome = "The authentication mutation was accepted but reconnect verification failed."
		}
		d.AddWarning("WebUI authentication reconnect is unverified", fmt.Sprintf("%s Resource state is retained. Configure provider credentials that can authenticate, refresh the actual username, then review the targeted plan. The password will not be replayed unless its version changes.", outcome))
	}
	return true
}
func (r *webuiAuthResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.State = req.State
	p, err := r.client.Preferences(ctx)
	if err != nil {
		resp.Diagnostics.AddWarning("WebUI authentication refresh unavailable", "The configured provider credentials cannot authenticate to qBittorrent. State was retained; configure credentials that can access the WebUI and refresh again. No write was sent.")
		return
	}
	if p.WebUIUsername == nil {
		resp.Diagnostics.AddError("WebUI username unavailable", "The server did not return its established nonsecret WebUI username; auth state was retained and no write was sent.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("username"), types.StringValue(*p.WebUIUsername))...)
}
func (r *webuiAuthResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
func (r *webuiAuthResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "application" {
		resp.Diagnostics.AddError("Invalid import ID", "WebUI auth import ID must be application.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

type webUIUsernameValidator struct{}

func (webUIUsernameValidator) Description(context.Context) string {
	return "must contain at least three characters and no colon"
}
func (webUIUsernameValidator) MarkdownDescription(ctx context.Context) string {
	return webUIUsernameValidator{}.Description(ctx)
}
func (webUIUsernameValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v := req.ConfigValue.ValueString()
	if nativeQStringLength(v) < 3 || strings.Contains(v, ":") {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid WebUI username", "qBittorrent requires at least three characters and does not allow a colon.")
	}
}

type webUIPasswordValidator struct{}

func (webUIPasswordValidator) Description(context.Context) string {
	return "must contain at least six characters"
}
func (webUIPasswordValidator) MarkdownDescription(ctx context.Context) string {
	return webUIPasswordValidator{}.Description(ctx)
}
func (webUIPasswordValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && nativeQStringLength(req.ConfigValue.ValueString()) < 6 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid WebUI password", "qBittorrent requires a password of at least six characters.")
	}
}

func nativeQStringLength(s string) int { return len(utf16.Encode([]rune(s))) }

var _ resource.Resource = (*webuiAuthResource)(nil)
var _ resource.ResourceWithImportState = (*webuiAuthResource)(nil)
