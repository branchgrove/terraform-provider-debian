package provider

import (
	"context"
	"fmt"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &FileDataSource{}
var _ datasource.DataSourceWithConfigure = &FileDataSource{}

const defaultMaxSize = 1048576 // 1 MiB

func NewFileDataSource() datasource.DataSource {
	return &FileDataSource{}
}

type FileDataSource struct {
	providerData *ProviderData
}

type FileDataSourceModel struct {
	Path       types.String    `tfsdk:"path"`
	MaxSize    types.Int64     `tfsdk:"max_size"`
	Content    types.String    `tfsdk:"content"`
	Owner      types.String    `tfsdk:"owner"`
	Group      types.String    `tfsdk:"group"`
	UID        types.Int64     `tfsdk:"uid"`
	GID        types.Int64     `tfsdk:"gid"`
	Mode       types.String    `tfsdk:"mode"`
	SHA256     types.String    `tfsdk:"sha256"`
	Size       types.Int64     `tfsdk:"size"`
	Basename   types.String    `tfsdk:"basename"`
	Dirname    types.String    `tfsdk:"dirname"`
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (d *FileDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_file"
}

func (d *FileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_file` reads a file's contents and metadata from the remote host.",

		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				MarkdownDescription: "Absolute path to the file.",
				Required:            true,
			},
			"max_size": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of bytes to read into state. Defaults to `1048576` (1 MiB).",
				Optional:            true,
				Computed:            true,
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "File contents as a string.",
				Computed:            true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "Owner name.",
				Computed:            true,
			},
			"group": schema.StringAttribute{
				MarkdownDescription: "Group name.",
				Computed:            true,
			},
			"uid": schema.Int64Attribute{
				MarkdownDescription: "Numeric owner ID.",
				Computed:            true,
			},
			"gid": schema.Int64Attribute{
				MarkdownDescription: "Numeric group ID.",
				Computed:            true,
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "Permission mode.",
				Computed:            true,
			},
			"sha256": schema.StringAttribute{
				MarkdownDescription: "SHA256 checksum.",
				Computed:            true,
			},
			"size": schema.Int64Attribute{
				MarkdownDescription: "Size in bytes.",
				Computed:            true,
			},
			"basename": schema.StringAttribute{
				MarkdownDescription: "Basename.",
				Computed:            true,
			},
			"dirname": schema.StringAttribute{
				MarkdownDescription: "Parent directory.",
				Computed:            true,
			},
			"ssh": dataSourceConnectionSchema,
		},
	}
}

func (d *FileDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FileDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Connection.ApplyDefaults()

	maxSize := int64(defaultMaxSize)
	if !data.MaxSize.IsNull() && !data.MaxSize.IsUnknown() {
		maxSize = data.MaxSize.ValueInt64()
	}
	data.MaxSize = types.Int64Value(maxSize)

	client, err := data.Connection.GetClient(ctx, d.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	file, err := client.GetFile(ctx, data.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read file metadata", err.Error())
		return
	}

	content, err := client.ReadFile(ctx, data.Path.ValueString(), int(maxSize))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read file content", err.Error())
		return
	}

	data.Content = types.StringValue(content)
	data.applyFileState(file)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (m *FileDataSourceModel) applyFileState(f *ssh.File) {
	m.Owner = types.StringValue(f.User)
	m.Group = types.StringValue(f.Group)
	m.Mode = types.StringValue(f.Mode)
	m.UID = types.Int64Value(int64(f.UID))
	m.GID = types.Int64Value(int64(f.GID))
	m.Basename = types.StringValue(f.Basename)
	m.Dirname = types.StringValue(f.Dirname)
	m.SHA256 = types.StringValue(f.SHA256)
	m.Size = types.Int64Value(int64(f.Size))
}
