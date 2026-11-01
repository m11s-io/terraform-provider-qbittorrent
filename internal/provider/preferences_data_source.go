package provider

import (
	"context"
	"encoding/json"
	"fmt"
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
	for _, f := range allPreferenceFields() {
		if f.excludeFromDataSource {
			continue
		}
		switch f.kind {
		case preferenceString:
			attrs[f.name] = schema.StringAttribute{Computed: true, MarkdownDescription: f.description}
		case preferenceBool:
			attrs[f.name] = schema.BoolAttribute{Computed: true, MarkdownDescription: f.description}
		case preferenceInt:
			attrs[f.name] = schema.Int64Attribute{Computed: true, MarkdownDescription: f.description}
		case preferenceFloat:
			attrs[f.name] = schema.Float64Attribute{Computed: true, MarkdownDescription: f.description}
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
	for _, f := range allPreferenceFields() {
		if f.excludeFromDataSource {
			continue
		}
		var v attr.Value
		switch f.kind {
		case preferenceString:
			v = types.StringNull()
		case preferenceBool:
			v = types.BoolNull()
		case preferenceInt:
			v = types.Int64Null()
		case preferenceFloat:
			v = types.Float64Null()
		}
		raw, ok := remote[f.wireKey]
		v, err = preferenceDataSourceValue(f, raw, ok)
		if err != nil {
			if !ok {
				resp.Diagnostics.AddError("Unsupported preferences response", "The server omitted preference "+f.name+".")
			} else {
				resp.Diagnostics.AddError("Invalid preferences response", "The server returned an invalid preference type.")
			}
			return
		}
		values[f.name] = v
	}
	obj, diags := types.ObjectValue(ats, values)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}

func preferenceDataSourceValue(f preferenceField, raw json.RawMessage, present bool) (attr.Value, error) {
	if !present {
		if f.capability == "" {
			return nil, fmt.Errorf("server omitted established preference %s", f.name)
		}
		switch f.kind {
		case preferenceString:
			return types.StringNull(), nil
		case preferenceBool:
			return types.BoolNull(), nil
		case preferenceInt:
			return types.Int64Null(), nil
		case preferenceFloat:
			return types.Float64Null(), nil
		default:
			return nil, fmt.Errorf("unsupported preference kind %q", f.kind)
		}
	}
	var zero attr.Value
	switch f.kind {
	case preferenceString:
		zero = types.StringValue("")
	case preferenceBool:
		zero = types.BoolValue(false)
	case preferenceInt:
		zero = types.Int64Value(0)
	case preferenceFloat:
		zero = types.Float64Value(0)
	default:
		return nil, fmt.Errorf("unsupported preference kind %q", f.kind)
	}
	return preferenceValue(zero, raw)
}
