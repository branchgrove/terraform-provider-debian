package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ReleaseDataSource{}
var _ datasource.DataSourceWithConfigure = &ReleaseDataSource{}

func NewReleaseDataSource() datasource.DataSource {
	return &ReleaseDataSource{}
}

type ReleaseDataSource struct {
	providerData *ProviderData
}

type ReleaseDataSourceModel struct {
	ID              types.String    `tfsdk:"id"`
	VersionID       types.String    `tfsdk:"version_id"`
	VersionCodename types.String    `tfsdk:"version_codename"`
	PrettyName      types.String    `tfsdk:"pretty_name"`
	Connection      ConnectionModel `tfsdk:"ssh"`
}

func (d *ReleaseDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_release"
}

func (d *ReleaseDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_release` reads the Debian release information from `/etc/os-release`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Distributor ID (for example, `debian`).",
				Computed:            true,
			},
			"version_id": schema.StringAttribute{
				MarkdownDescription: "Version number (for example, `12`).",
				Computed:            true,
			},
			"version_codename": schema.StringAttribute{
				MarkdownDescription: "Version codename (for example, `bookworm`).",
				Computed:            true,
			},
			"pretty_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name (for example, `Debian GNU/Linux 12 (bookworm)`).",
				Computed:            true,
			},
			"ssh": dataSourceConnectionSchema,
		},
	}
}

func (d *ReleaseDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.providerData = pd
}

func (d *ReleaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ReleaseDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Connection.ApplyDefaults()

	client, err := data.Connection.GetClient(ctx, d.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	res, err := client.Run(ctx, "cat /etc/os-release", nil, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read /etc/os-release", err.Error())
		return
	}

	fields := parseOSRelease(string(res.Stdout))

	data.ID = types.StringValue(fields["ID"])
	data.VersionID = types.StringValue(fields["VERSION_ID"])
	data.VersionCodename = types.StringValue(fields["VERSION_CODENAME"])
	data.PrettyName = types.StringValue(fields["PRETTY_NAME"])

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// parseOSRelease parses /etc/os-release content into a key-value map.
// Values may be unquoted, double-quoted, or single-quoted.
func parseOSRelease(content string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			continue
		}
		key := line[:eqIdx]
		val := line[eqIdx+1:]
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		result[key] = val
	}
	return result
}
