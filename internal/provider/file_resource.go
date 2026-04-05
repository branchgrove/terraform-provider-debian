package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
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
	// providerData carries the shared SSH connection pool and provider-level
	// authentication (default private key and key ring). Resources use it as
	// a fallback when their ssh block does not specify auth directly.
	providerData *ProviderData
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
	Overwrite         types.Bool      `tfsdk:"overwrite"`
	Connection        ConnectionModel `tfsdk:"ssh"`
}

func (r *FileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_file"
}

func (r *FileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_file` manages a file on the debian host.",

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
				MarkdownDescription: "File permission mode, for example, `0644`. Defaults to `0666` before umask is applied.",
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
			"overwrite": schema.BoolAttribute{
				MarkdownDescription: "Overwrite the file if it already exists. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"ssh": connectionSchema,
		},
	}
}

func (r *FileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FileResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	_, err = client.GetFile(ctx, data.Path.ValueString())
	if err == nil {
		if !data.Overwrite.ValueBool() {
			resp.Diagnostics.AddError("Resource already exists", "The file already exists and overwrite is false")
			return
		}
	} else if !errors.Is(err, ssh.ErrNotFound) {
		resp.Diagnostics.AddError("Failed to check if file exists", err.Error())
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

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	previousSha := data.SHA256.ValueString()

	file, err := client.GetFile(ctx, data.Path.ValueString())
	if err != nil {
		if errors.Is(err, ssh.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read file", err.Error())
		return
	}

	data.applyFileState(file)

	// When the remote SHA changes, the file was modified out of band. Null out
	// content so Terraform re-applies it from config on the next apply. This
	// avoids reading file content over SSH on every refresh.
	if !data.Content.IsNull() && data.SHA256.ValueString() != previousSha {
		data.Content = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FileResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
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

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	if err := client.DeleteFile(ctx, data.Path.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete file", err.Error())
	}
}

// ImportState parses a composite import ID and populates the ssh connection and
// path in state so that Read can connect and fetch the file metadata.
//
// Format: user=<user>;host=<host>;port=<port>[;public_key=<key>];id=<path>
//
// The "user" key defaults to "root" when omitted. Values are percent-decoded
// so that literal semicolons can be represented as %3B.
func (r *FileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	user, publicKey, host, port, filePath, err := parseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	if filePath == "" || filePath[0] != '/' {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("file path must be absolute, got %q", filePath))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), filePath)...)

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

// parseImportID extracts connection and resource-specific ID from a composite
// import string.
//
// Format: user=<user>;host=<host>;port=<port>[;public_key=<key>];id=<resource-id>
//
// Values are percent-decoded so that literal semicolons (%3B) are handled
// correctly. The "user" key is optional and defaults to "root".
func parseImportID(id string) (user, publicKey, host string, port int, resourceID string, err error) {
	params := make(map[string]string)
	for _, pair := range strings.Split(id, ";") {
		eqIdx := strings.Index(pair, "=")
		if eqIdx == -1 {
			return "", "", "", 0, "", fmt.Errorf("invalid import ID %q: expected key=value pair, got %q", id, pair)
		}
		key := pair[:eqIdx]
		val, decErr := url.PathUnescape(pair[eqIdx+1:])
		if decErr != nil {
			return "", "", "", 0, "", fmt.Errorf("invalid import ID %q: decode value for %q: %w", id, key, decErr)
		}
		params[key] = val
	}

	host = params["host"]
	if host == "" {
		return "", "", "", 0, "", fmt.Errorf("invalid import ID %q: missing required key \"host\"", id)
	}

	portStr := params["port"]
	if portStr == "" {
		return "", "", "", 0, "", fmt.Errorf("invalid import ID %q: missing required key \"port\"", id)
	}
	port, err = strconv.Atoi(portStr)
	if err != nil {
		return "", "", "", 0, "", fmt.Errorf("invalid import ID %q: invalid port %q: %w", id, portStr, err)
	}

	resourceID = params["id"]
	if resourceID == "" {
		return "", "", "", 0, "", fmt.Errorf("invalid import ID %q: missing required key \"id\"", id)
	}

	user = params["user"]
	if user == "" {
		user = "root"
	}

	publicKey = params["public_key"]
	return user, publicKey, host, port, resourceID, nil
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
