package provider

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

type preferencesDataSource struct{ client *client.Client }

func (d *preferencesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_preferences"
}
func (d *preferencesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Singleton ID: application."}}
	for _, f := range preferenceFields {
		switch f.kind {
		case "string":
			attrs[f.name] = schema.StringAttribute{Computed: true, MarkdownDescription: f.description}
		case "bool":
			attrs[f.name] = schema.BoolAttribute{Computed: true, MarkdownDescription: f.description}
		default:
			attrs[f.name] = schema.Int64Attribute{Computed: true, MarkdownDescription: f.description}
		}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Read selected nonsecret qBittorrent preferences. Never returns passwords, hashes, API keys or session cookies.", Attributes: attrs}
}
func (d *preferencesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	d.client, ok = req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected qBittorrent client.")
	}
}
func (d *preferencesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	p, err := d.client.Preferences(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read preferences", err.Error())
		return
	}
	b, _ := json.Marshal(p)
	var remote map[string]json.RawMessage
	json.Unmarshal(b, &remote)
	ats := resp.State.Schema.Type().(types.ObjectType).AttrTypes
	values := map[string]attr.Value{"id": types.StringValue("application")}
	for _, f := range preferenceFields {
		var v attr.Value
		switch f.kind {
		case "string":
			v = types.StringNull()
		case "bool":
			v = types.BoolNull()
		default:
			v = types.Int64Null()
		}
		if raw, ok := remote[f.name]; ok {
			v, err = preferenceValue(v, raw)
			if err != nil {
				resp.Diagnostics.AddError("Invalid preferences response", "The server returned an invalid preference type.")
				return
			}
		} else {
			resp.Diagnostics.AddError("Unsupported preferences response", "The server omitted preference "+f.name+".")
			return
		}
		values[f.name] = v
	}
	obj, diags := types.ObjectValue(ats, values)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
