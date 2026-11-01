package provider

import (
	"context"
	"encoding/json"
	"fmt"

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

type credentialsResource struct{ client *client.Client }

func (r *credentialsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_credentials"
}

func (r *credentialsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Singleton ID: application.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
	}
	for _, secret := range credentialSecrets {
		attrs[secret.valueAttribute] = schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: secret.description}
		attrs[secret.versionAttribute] = schema.Int64Attribute{Optional: true, MarkdownDescription: "Increment this counter when rotating the corresponding write-only password. The password is sent only when this version changes.", Validators: []validator.Int64{nonNegativeVersionValidator{}}}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Singleton write-only management of qBittorrent SMTP, proxy and DynDNS passwords. Import ID: application. Configure a password and its version together; increment only the relevant version to rotate a password. An empty password explicitly clears it when the native setter supports clearing. Removing both fields relinquishes management without changing the stored password. Read and destroy never retrieve or mutate passwords.",
		Attributes:          attrs,
	}
}

func (r *credentialsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	r.client, ok = req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected qBittorrent client.")
	}
}

func (r *credentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var cfg, plan types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := buildVersionedSecretPatch(cfg, plan, types.ObjectNull(plan.AttributeTypes(ctx)), true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, patch, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
}

func (r *credentialsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var cfg, plan, prior types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := buildVersionedSecretPatch(cfg, plan, prior, false, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, patch, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State = tfsdk.State{Schema: req.Plan.Schema, Raw: req.Plan.Raw}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "application")...)
}

func (r *credentialsResource) write(ctx context.Context, values map[string]any, d *diag.Diagnostics) {
	if len(values) == 0 {
		return
	}
	b, err := json.Marshal(values)
	if err != nil {
		d.AddError("Cannot encode credentials", "Credential patch could not be encoded.")
		return
	}
	var patch client.CredentialsPatch
	if err := json.Unmarshal(b, &patch); err != nil {
		d.AddError("Cannot encode credentials", "Credential fields do not match the client contract.")
		return
	}
	if err := r.client.SetCredentialsPatch(ctx, patch); err != nil {
		d.AddError("Cannot update credentials", err.Error())
	}
}

func (r *credentialsResource) Read(_ context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Credential readback is intentionally impossible: preserve only nonsecret counters.
	resp.State = req.State
}

func (r *credentialsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *credentialsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "application" {
		resp.Diagnostics.AddError("Invalid import ID", "Credentials resource import ID must be application.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

type versionedSecretSpec struct {
	valueAttribute, versionAttribute, wireKey, description string
}

// credentialSecrets is also the field contract shared with the WebUI auth
// resource. Keep values out of plan/state and version counters in state.
var credentialSecrets = []versionedSecretSpec{
	{valueAttribute: "smtp_password_wo", versionAttribute: "smtp_password_wo_version", wireKey: "mail_notification_password", description: "SMTP password (write-only). Set together with smtp_password_wo_version; empty string clears it."},
	{valueAttribute: "proxy_password_wo", versionAttribute: "proxy_password_wo_version", wireKey: "proxy_password", description: "Proxy password (write-only). Set together with proxy_password_wo_version; empty string clears it."},
	{valueAttribute: "dyndns_password_wo", versionAttribute: "dyndns_password_wo_version", wireKey: "dyndns_password", description: "Dynamic DNS password (write-only). Set together with dyndns_password_wo_version; empty string clears it."},
}

// buildVersionedSecretPatch selects only secrets whose nonsecret version changed.
// The WebUI authentication resource should reuse this helper with its own spec.
func buildVersionedSecretPatch(config, plan, prior types.Object, creating bool, d *diag.Diagnostics) map[string]any {
	patch := map[string]any{}
	for _, secret := range credentialSecrets {
		wire, value, include := versionedSecretPatch(config, plan, prior, secret, creating, d)
		if include {
			patch[wire] = value
		}
	}
	return patch
}

func versionedSecretPatch(config, plan, prior types.Object, secret versionedSecretSpec, creating bool, d *diag.Diagnostics) (string, string, bool) {
	configValues, planValues := config.Attributes(), plan.Attributes()
	secretValue, ok := configValues[secret.valueAttribute].(types.String)
	if !ok {
		d.AddAttributeError(path.Root(secret.valueAttribute), "Invalid write-only secret", "The configured write-only password is unavailable.")
		return "", "", false
	}
	version, ok := planValues[secret.versionAttribute].(types.Int64)
	if !ok {
		d.AddAttributeError(path.Root(secret.versionAttribute), "Invalid secret version", "The password version must be a known integer.")
		return "", "", false
	}
	if secretValue.IsUnknown() || version.IsUnknown() {
		d.AddAttributeError(path.Root(secret.versionAttribute), "Unknown secret input", "The password and its version must be known at apply.")
		return "", "", false
	}
	if secretValue.IsNull() && version.IsNull() {
		return secret.wireKey, "", false
	}
	if secretValue.IsNull() != version.IsNull() {
		d.AddAttributeError(path.Root(secret.valueAttribute), "Incomplete secret rotation", fmt.Sprintf("Configure %s and %s together, or set both to null to relinquish management.", secret.valueAttribute, secret.versionAttribute))
		return "", "", false
	}
	if creating {
		return secret.wireKey, secretValue.ValueString(), true
	}
	priorVersion, exists := prior.Attributes()[secret.versionAttribute].(types.Int64)
	if !exists || priorVersion.IsNull() {
		return secret.wireKey, secretValue.ValueString(), true
	}
	if version.ValueInt64() < priorVersion.ValueInt64() {
		d.AddAttributeError(path.Root(secret.versionAttribute), "Secret version must increase", "Increment the version counter when rotating a password; it cannot decrease.")
		return "", "", false
	}
	if version.ValueInt64() == priorVersion.ValueInt64() {
		return secret.wireKey, "", false
	}
	return secret.wireKey, secretValue.ValueString(), true
}

var _ resource.Resource = (*credentialsResource)(nil)
var _ resource.ResourceWithImportState = (*credentialsResource)(nil)

type nonNegativeVersionValidator struct{}

func (nonNegativeVersionValidator) Description(context.Context) string {
	return "must be a nonnegative integer"
}
func (nonNegativeVersionValidator) MarkdownDescription(ctx context.Context) string {
	return nonNegativeVersionValidator{}.Description(ctx)
}
func (nonNegativeVersionValidator) ValidateInt64(_ context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && req.ConfigValue.ValueInt64() < 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid secret version", "The version counter must be nonnegative.")
	}
}
