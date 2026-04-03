package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &DirectoryResource{}
var _ resource.ResourceWithImportState = &DirectoryResource{}

func NewDirectoryResource() resource.Resource {
	return &DirectoryResource{}
}

type DirectoryResource struct {
	providerData *ProviderData
}

type DirectoryResourceModel struct {
	Path          types.String    `tfsdk:"path"`
	Owner         types.String    `tfsdk:"owner"`
	Group         types.String    `tfsdk:"group"`
	Mode          types.String    `tfsdk:"mode"`
	CreateParents types.Bool      `tfsdk:"create_parents"`
	UID           types.Int64     `tfsdk:"uid"`
	GID           types.Int64     `tfsdk:"gid"`
	Basename      types.String    `tfsdk:"basename"`
	Dirname       types.String    `tfsdk:"dirname"`
	Connection    ConnectionModel `tfsdk:"ssh"`
}

func (r *DirectoryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_directory"
}

func (r *DirectoryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_directory` manages a directory on the debian host.",

		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				MarkdownDescription: "Absolute path to the directory.",
				Required:            true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "Owner of the directory. Conflicts with `uid`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("uid")),
				},
			},
			"group": schema.StringAttribute{
				MarkdownDescription: "Group of the directory. Conflicts with `gid`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("gid")),
				},
			},
			"uid": schema.Int64Attribute{
				MarkdownDescription: "Numeric user ID of the directory owner. Conflicts with `owner`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.ConflictsWith(path.MatchRoot("owner")),
				},
			},
			"gid": schema.Int64Attribute{
				MarkdownDescription: "Numeric group ID of the directory group. Conflicts with `group`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.ConflictsWith(path.MatchRoot("group")),
				},
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "Directory permission mode, for example, `0755`. Defaults to `0755`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("0755"),
			},
			"create_parents": schema.BoolAttribute{
				MarkdownDescription: "Create parent directories if they don't exist. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"basename": schema.StringAttribute{
				MarkdownDescription: "Basename of the directory.",
				Computed:            true,
			},
			"dirname": schema.StringAttribute{
				MarkdownDescription: "Parent directory path.",
				Computed:            true,
			},
			"ssh": connectionSchema,
		},
	}
}

func (r *DirectoryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.providerData = pd
}

func (r *DirectoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DirectoryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	dir, err := client.MakeDirectory(ctx, data.toMakeDirectoryCommand())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create directory", err.Error())
		return
	}

	data.applyDirectoryState(dir)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DirectoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DirectoryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	dir, err := client.GetDirectory(ctx, data.Path.ValueString())
	if err != nil {
		if errors.Is(err, ssh.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read directory", err.Error())
		return
	}

	data.applyDirectoryState(dir)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DirectoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DirectoryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	dir, err := client.UpdateDirectory(ctx, data.toMakeDirectoryCommand())
	if err != nil {
		resp.Diagnostics.AddError("Failed to update directory", err.Error())
		return
	}

	data.applyDirectoryState(dir)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DirectoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DirectoryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	if err := client.DeleteDirectory(ctx, data.Path.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete directory", err.Error())
	}
}

func (r *DirectoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	user, publicKey, host, port, dirPath, err := parseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	if dirPath == "" || dirPath[0] != '/' {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("directory path must be absolute, got %q", dirPath))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), dirPath)...)

	conn := ConnectionModel{
		Hostname: types.StringValue(host),
		Port:     types.Int32Value(int32(port)),
		User:     types.StringValue(user),
	}
	if publicKey != "" {
		conn.PublicKey = types.StringValue(publicKey)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ssh"), conn)...)
}

func (m *DirectoryResourceModel) toMakeDirectoryCommand() *ssh.MakeDirectoryCommand {
	cmd := &ssh.MakeDirectoryCommand{
		Path:          m.Path.ValueString(),
		Mode:          m.Mode.ValueString(),
		CreateParents: m.CreateParents.ValueBool(),
	}

	if !m.Owner.IsNull() && !m.Owner.IsUnknown() {
		cmd.User = m.Owner.ValueString()
	}
	if !m.Group.IsNull() && !m.Group.IsUnknown() {
		cmd.Group = m.Group.ValueString()
	}
	if !m.UID.IsNull() && !m.UID.IsUnknown() {
		uid := int(m.UID.ValueInt64())
		cmd.UID = &uid
	}
	if !m.GID.IsNull() && !m.GID.IsUnknown() {
		gid := int(m.GID.ValueInt64())
		cmd.GID = &gid
	}

	return cmd
}

func (m *DirectoryResourceModel) applyDirectoryState(d *ssh.Directory) {
	m.Owner = types.StringValue(d.User)
	m.Group = types.StringValue(d.Group)
	m.Mode = types.StringValue(d.Mode)
	m.UID = types.Int64Value(int64(d.UID))
	m.GID = types.Int64Value(int64(d.GID))
	m.Basename = types.StringValue(d.Basename)
	m.Dirname = types.StringValue(d.Dirname)
}
