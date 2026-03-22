package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

type UserResource struct {
	providerData *ProviderData
}

type UserResourceModel struct {
	Name       types.String    `tfsdk:"name"`
	UID        types.Int64     `tfsdk:"uid"`
	GID        types.Int64     `tfsdk:"gid"`
	Group      types.String    `tfsdk:"group"`
	Home       types.String    `tfsdk:"home"`
	Shell      types.String    `tfsdk:"shell"`
	System     types.Bool      `tfsdk:"system"`
	CreateHome types.Bool      `tfsdk:"create_home"`
	Groups     types.Set       `tfsdk:"groups"`
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_user` manages a local user account on the remote host.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Username. Changing this forces recreation.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"uid": schema.Int64Attribute{
				MarkdownDescription: "Numeric user ID. If omitted, the system assigns one.",
				Optional:            true,
				Computed:            true,
			},
			"gid": schema.Int64Attribute{
				MarkdownDescription: "Primary group ID. Conflicts with `group`.",
				Optional:            true,
				Computed:            true,
			},
			"group": schema.StringAttribute{
				MarkdownDescription: "Primary group name. Conflicts with `gid`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("gid")),
				},
			},
			"home": schema.StringAttribute{
				MarkdownDescription: "Home directory path.",
				Optional:            true,
				Computed:            true,
			},
			"shell": schema.StringAttribute{
				MarkdownDescription: "Login shell.",
				Optional:            true,
				Computed:            true,
			},
			"system": schema.BoolAttribute{
				MarkdownDescription: "Create as system user. Defaults to `false`. Changing this forces recreation.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"create_home": schema.BoolAttribute{
				MarkdownDescription: "Create home directory. Defaults to `true`. Changing this forces recreation.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"groups": schema.SetAttribute{
				MarkdownDescription: "Supplementary group names. Overwrites all supplementary memberships. When null, memberships are not managed.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"ssh": connectionSchema,
		},
	}
}

func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T.", req.ProviderData),
		)
		return
	}

	r.providerData = pd
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	cmd := &ssh.CreateUserCommand{
		Name:   data.Name.ValueString(),
		System: data.System.ValueBool(),
	}
	if !data.UID.IsNull() && !data.UID.IsUnknown() {
		uid := int(data.UID.ValueInt64())
		cmd.UID = &uid
	}
	if !data.GID.IsNull() && !data.GID.IsUnknown() {
		gid := int(data.GID.ValueInt64())
		cmd.GID = &gid
	} else if !data.Group.IsNull() && !data.Group.IsUnknown() {
		cmd.Group = data.Group.ValueString()
	}
	if !data.Home.IsNull() && !data.Home.IsUnknown() {
		cmd.Home = data.Home.ValueString()
	}
	if !data.Shell.IsNull() && !data.Shell.IsUnknown() {
		cmd.Shell = data.Shell.ValueString()
	}
	createHome := data.CreateHome.ValueBool()
	cmd.CreateHome = &createHome

	if !data.Groups.IsNull() {
		var groups []string
		resp.Diagnostics.Append(data.Groups.ElementsAs(ctx, &groups, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		cmd.Groups = groups
	}

	user, err := client.CreateUser(ctx, cmd)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create user", err.Error())
		return
	}

	data.applyUserState(ctx, user, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	user, err := client.GetUser(ctx, data.Name.ValueString())
	if err != nil {
		if errors.Is(err, ssh.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read user", err.Error())
		return
	}

	data.applyUserState(ctx, user, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	cmd := &ssh.UpdateUserCommand{
		Name: data.Name.ValueString(),
	}
	if !data.UID.IsNull() && !data.UID.IsUnknown() {
		uid := int(data.UID.ValueInt64())
		cmd.UID = &uid
	}
	if !data.GID.IsNull() && !data.GID.IsUnknown() {
		gid := int(data.GID.ValueInt64())
		cmd.GID = &gid
	} else if !data.Group.IsNull() && !data.Group.IsUnknown() {
		cmd.Group = data.Group.ValueString()
	}
	if !data.Home.IsNull() && !data.Home.IsUnknown() {
		cmd.Home = data.Home.ValueString()
	}
	if !data.Shell.IsNull() && !data.Shell.IsUnknown() {
		cmd.Shell = data.Shell.ValueString()
	}
	if !data.Groups.IsNull() {
		var groups []string
		resp.Diagnostics.Append(data.Groups.ElementsAs(ctx, &groups, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		cmd.Groups = &groups
	}

	user, err := client.UpdateUser(ctx, cmd)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update user", err.Error())
		return
	}

	data.applyUserState(ctx, user, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	if err := client.DeleteUser(ctx, data.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete user", err.Error())
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	user, publicKey, host, port, userName, err := parseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	if userName == "" {
		resp.Diagnostics.AddError("Invalid import ID", "username must not be empty")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), userName)...)

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

func (m *UserResourceModel) applyUserState(ctx context.Context, u *ssh.User, diags *diag.Diagnostics) {
	m.Name = types.StringValue(u.Name)
	m.UID = types.Int64Value(int64(u.UID))
	m.GID = types.Int64Value(int64(u.GID))
	m.Group = types.StringValue(u.Group)
	m.Home = types.StringValue(u.Home)
	m.Shell = types.StringValue(u.Shell)

	if !m.Groups.IsNull() {
		groups := u.Groups
		if groups == nil {
			groups = []string{}
		}
		groupsVal, d := types.SetValueFrom(ctx, types.StringType, groups)
		diags.Append(d...)
		m.Groups = groupsVal
	}
}
