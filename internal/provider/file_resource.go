package provider

import (
	"context"
	"fmt"
	"strings"

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

var _ resource.Resource = &FileResource{}
var _ resource.ResourceWithImportState = &FileResource{}

func NewFileResource() resource.Resource {
	return &FileResource{}
}

type FileResource struct {
	sshManager *ssh.Manager
}

type FileResourceModel struct {
	Path              types.String    `tfsdk:"path"`
	Content           types.String    `tfsdk:"content"`
	Owner             types.String    `tfsdk:"owner"`
	Group             types.String    `tfsdk:"group"`
	Mode              types.String    `tfsdk:"mode"`
	CreateDirectories types.Bool      `tfsdk:"create_directories"`
	UID               types.Int64     `tfsdk:"uid"`
	GID               types.Int64     `tfsdk:"gid"`
	Basename          types.String    `tfsdk:"basename"`
	Dirname           types.String    `tfsdk:"dirname"`
	SHA256            types.String    `tfsdk:"sha256"`
	Size              types.Int64     `tfsdk:"size"`
	Connection        ConnectionModel `tfsdk:"ssh"`
}

func (r *FileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_file"
}

func (r *FileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`system_file` manages a file on the debian host.",

		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				MarkdownDescription: "Absolute path to the file.",
				Required:            true,
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "Content of the file.",
				// TODO: set to optional and allow "source" for path to local file or "template" which is an object containing "source" and "vars"
				Required: true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "Owner of the file. Conflicts with `uid`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("uid")),
				},
			},
			"group": schema.StringAttribute{
				MarkdownDescription: "Group of the file. Conflicts with `gid`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("gid")),
				},
			},
			"uid": schema.Int64Attribute{
				MarkdownDescription: "Numeric user ID of the file owner. Conflicts with `owner`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.ConflictsWith(path.MatchRoot("owner")),
				},
			},
			"gid": schema.Int64Attribute{
				MarkdownDescription: "Numeric group ID of the file group. Conflicts with `group`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.ConflictsWith(path.MatchRoot("group")),
				},
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "File permission mode, e.g. `0644`. Defaults to `0666` before umask is applied.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("0644"),
			},
			"create_directories": schema.BoolAttribute{
				MarkdownDescription: "Create parent directories if they don't exist. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"basename": schema.StringAttribute{
				MarkdownDescription: "Basename of the file.",
				Computed:            true,
			},
			"dirname": schema.StringAttribute{
				MarkdownDescription: "Directory name of the file.",
				Computed:            true,
			},
			"sha256": schema.StringAttribute{
				MarkdownDescription: "The sha256 checksum of the contents of the file.",
				Computed:            true,
			},
			"size": schema.Int64Attribute{
				MarkdownDescription: "Size of the file in bytes.",
				Computed:            true,
			},
			"ssh": connectionSchema,
		},
	}
}

func (r *FileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	sshManager, ok := req.ProviderData.(*ssh.Manager)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.sshManager = sshManager
}

func (r *FileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FileResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.sshManager)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	file, err := client.PutFile(ctx, data.toPutFileCommand())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create file", err.Error())
		return
	}

	data.applyFileState(file)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FileResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.sshManager)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	file, err := client.GetFile(ctx, data.Path.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.applyFileState(file)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FileResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.sshManager)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	file, err := client.PutFile(ctx, data.toPutFileCommand())
	if err != nil {
		resp.Diagnostics.AddError("Failed to update file", err.Error())
		return
	}

	data.applyFileState(file)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FileResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.sshManager)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	if err := client.DeleteFile(ctx, data.Path.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete file", err.Error())
	}
}

func (r *FileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("path"), req, resp)
}

// toPutFileCommand converts the Terraform model to an ssh.PutFileCommand,
// bridging Terraform's tri-state types to plain Go values. Only fields the
// user actually set are populated so the SSH layer applies defaults correctly.
func (m *FileResourceModel) toPutFileCommand() *ssh.PutFileCommand {
	cmd := &ssh.PutFileCommand{
		Path:              m.Path.ValueString(),
		Content:           strings.NewReader(m.Content.ValueString()),
		Mode:              m.Mode.ValueString(),
		CreateDirectories: m.CreateDirectories.ValueBool(),
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

// applyFileState writes the remote file's metadata back into the Terraform
// model so that computed attributes reflect the actual state on disk.
func (m *FileResourceModel) applyFileState(f *ssh.File) {
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
