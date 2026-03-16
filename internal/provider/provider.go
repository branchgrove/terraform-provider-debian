package provider

import (
	"context"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ provider.Provider = &DebianProvider{}
var _ provider.ProviderWithFunctions = &DebianProvider{}
var _ provider.ProviderWithEphemeralResources = &DebianProvider{}
var _ provider.ProviderWithActions = &DebianProvider{}

type DebianProvider struct {
	version string
}

type DebianProviderModel struct{}

func (p *DebianProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "debian"
	resp.Version = p.version
}

func (p *DebianProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{},
	}
}

func (p *DebianProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data DebianProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	sshManager := ssh.NewManager()

	resp.DataSourceData = sshManager
	resp.ResourceData = sshManager
}

func (p *DebianProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *DebianProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *DebianProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *DebianProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func (p *DebianProvider) Actions(ctx context.Context) []func() action.Action {
	return []func() action.Action{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &DebianProvider{
			version: version,
		}
	}
}
