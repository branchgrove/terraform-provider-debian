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
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &DebianProvider{}
var _ provider.ProviderWithFunctions = &DebianProvider{}
var _ provider.ProviderWithEphemeralResources = &DebianProvider{}
var _ provider.ProviderWithActions = &DebianProvider{}

type DebianProvider struct {
	version string
}

type DebianProviderModel struct {
	PrivateKey  types.String `tfsdk:"private_key"`
	PrivateKeys types.Map    `tfsdk:"private_keys"`
}

// ProviderData is passed to resources via Configure. It carries the shared SSH
// connection pool and the provider-level authentication. Resources define their
// own connection details (hostname, port, user, host_key) but fall back to the
// provider for authentication when no per-resource auth is specified.
type ProviderData struct {
	SSHManager  *ssh.Manager
	PrivateKey  string            // default private key, used when the resource ssh block omits auth
	PrivateKeys map[string]string // public_key -> private_key, looked up when a resource sets public_key
}

func (p *DebianProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "debian"
	resp.Version = p.version
}

func (p *DebianProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Debian provider manages resources on remote Debian hosts over SSH.",
		Attributes: map[string]schema.Attribute{
			"private_key": schema.StringAttribute{
				MarkdownDescription: "Default SSH private key used to authenticate when a resource's `ssh` block does not specify `private_key`, `public_key`, or `password`.",
				Optional:            true,
				Sensitive:           true,
			},
			"private_keys": schema.MapAttribute{
				MarkdownDescription: "A map of SSH public keys to their corresponding private keys. Resources can reference a public key in their `ssh` block to look up the private key for authentication.",
				Optional:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (p *DebianProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data DebianProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pd := &ProviderData{
		SSHManager:  ssh.NewManager(),
		PrivateKeys: make(map[string]string),
	}

	if !data.PrivateKey.IsNull() {
		pd.PrivateKey = data.PrivateKey.ValueString()
	}

	if !data.PrivateKeys.IsNull() {
		diags := data.PrivateKeys.ElementsAs(ctx, &pd.PrivateKeys, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.DataSourceData = pd
	resp.ResourceData = pd
	resp.EphemeralResourceData = pd
	resp.ActionData = pd
}

func (p *DebianProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewFileResource,
		NewDirectoryResource,
		NewGroupResource,
		NewUserResource,
		NewGroupMemberResource,
		NewAptPackagesResource,
	}
}

func (p *DebianProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewCommandEphemeral,
		NewFileEphemeral,
	}
}

func (p *DebianProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewCommandDataSource,
		NewFileDataSource,
		NewReleaseDataSource,
	}
}

func (p *DebianProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func (p *DebianProvider) Actions(ctx context.Context) []func() action.Action {
	return []func() action.Action{
		NewCommandAction,
		NewAptUpdateAction,
		NewAptUpgradeAction,
		NewSystemctlReloadAction,
		NewSystemctlRestartAction,
		NewSystemctlDaemonReloadAction,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &DebianProvider{
			version: version,
		}
	}
}
