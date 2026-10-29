package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client"
)

type qbittorrentProvider struct{ version string }
type providerModel struct {
	URL      types.String `tfsdk:"url"`
	APIKey   types.String `tfsdk:"api_key"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &qbittorrentProvider{version: version} }
}
func (p *qbittorrentProvider) Metadata(_ context.Context, _ provider.MetadataRequest, r *provider.MetadataResponse) {
	r.TypeName = "qbittorrent"
	r.Version = p.version
}
func (p *qbittorrentProvider) Schema(_ context.Context, _ provider.SchemaRequest, r *provider.SchemaResponse) {
	r.Schema = schema.Schema{
		MarkdownDescription: "Manage selected preferences and categories on qBittorrent 5.3.0 (Web API 2.16.2). Credentials must already exist on the server. TLS certificates are verified.",
		Attributes: map[string]schema.Attribute{
			"url":      schema.StringAttribute{Optional: true, MarkdownDescription: "Absolute HTTP(S) WebUI URL, including any proxy base path. Required here or through QBITTORRENT_URL. No embedded credentials, query or fragment."},
			"api_key":  schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Existing Bearer API key. Defaults to QBITTORRENT_API_KEY. Mutually exclusive with username/password."},
			"username": schema.StringAttribute{Optional: true, MarkdownDescription: "Session username. Defaults to QBITTORRENT_USERNAME."},
			"password": schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Session password. Defaults to QBITTORRENT_PASSWORD. Cookies remain in memory and never enter state."},
		}}
}
func (p *qbittorrentProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var m providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	values := map[string]types.String{"url": m.URL, "api_key": m.APIKey, "username": m.Username, "password": m.Password}
	env := map[string]string{"url": "QBITTORRENT_URL", "api_key": "QBITTORRENT_API_KEY", "username": "QBITTORRENT_USERNAME", "password": "QBITTORRENT_PASSWORD"}
	resolved := map[string]string{}
	for k, v := range values {
		if v.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root(k), "Unknown provider configuration", "This value must be known before the provider can connect.")
			continue
		}
		if v.IsNull() {
			resolved[k] = os.Getenv(env[k])
		} else {
			resolved[k] = v.ValueString()
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := client.New(client.Config{URL: resolved["url"], APIKey: resolved["api_key"], Username: resolved["username"], Password: resolved["password"]})
	if err != nil {
		resp.Diagnostics.AddError("Invalid qBittorrent provider configuration", err.Error())
		return
	}
	if err = c.CheckVersion(ctx); err != nil {
		resp.Diagnostics.AddError("Cannot configure qBittorrent provider", err.Error())
		return
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}
func (p *qbittorrentProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{func() resource.Resource { return &preferencesResource{} }, func() resource.Resource { return &categoryResource{} }}
}
func (p *qbittorrentProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{func() datasource.DataSource { return &preferencesDataSource{} }}
}
