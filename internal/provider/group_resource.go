package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &GroupResource{}
var _ resource.ResourceWithImportState = &GroupResource{}

func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

type GroupResource struct {
	providerData *ProviderData
}

type GroupResourceModel struct {
	Name       types.String    `tfsdk:"name"`
	GID        types.Int64     `tfsdk:"gid"`
	System     types.Bool      `tfsdk:"system"`
	Users      types.Set       `tfsdk:"users"`
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (r *GroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *GroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_group` manages a local group on the remote host.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Group name. Changing this forces recreation.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"gid": schema.Int64Attribute{
				MarkdownDescription: "Numeric group ID. If omitted, the system assigns one.",
				Optional:            true,
				Computed:            true,
			},
			"system": schema.BoolAttribute{
				MarkdownDescription: "Create as system group. Defaults to `false`. Changing this forces recreation.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"users": schema.SetAttribute{
				MarkdownDescription: "Members of this group. Overwrites the full member list. When null, membership is not managed.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"ssh": connectionSchema,
		},
	}
}

func (r *GroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	cmd := &ssh.CreateGroupCommand{
		Name:   data.Name.ValueString(),
		System: data.System.ValueBool(),
	}
	if !data.GID.IsNull() && !data.GID.IsUnknown() {
		gid := int(data.GID.ValueInt64())
		cmd.GID = &gid
	}

	group, err := client.CreateGroup(ctx, cmd)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create group", err.Error())
		return
	}

	if !data.Users.IsNull() {
		var users []string
		resp.Diagnostics.Append(data.Users.ElementsAs(ctx, &users, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if err := client.SetGroupMembers(ctx, group.Name, users); err != nil {
			resp.Diagnostics.AddError("Failed to set group members", err.Error())
			return
		}
		group, err = client.GetGroup(ctx, group.Name)
		if err != nil {
			resp.Diagnostics.AddError("Failed to read group", err.Error())
			return
		}
	}

	data.applyGroupState(ctx, group, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	group, err := client.GetGroup(ctx, data.Name.ValueString())
	if err != nil {
		if errors.Is(err, ssh.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read group", err.Error())
		return
	}

	data.applyGroupState(ctx, group, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	var gid *int
	if !data.GID.IsNull() && !data.GID.IsUnknown() {
		g := int(data.GID.ValueInt64())
		gid = &g
	}

	group, err := client.UpdateGroup(ctx, data.Name.ValueString(), gid)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update group", err.Error())
		return
	}

	if !data.Users.IsNull() {
		var users []string
		resp.Diagnostics.Append(data.Users.ElementsAs(ctx, &users, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if err := client.SetGroupMembers(ctx, group.Name, users); err != nil {
			resp.Diagnostics.AddError("Failed to set group members", err.Error())
			return
		}
		group, err = client.GetGroup(ctx, group.Name)
		if err != nil {
			resp.Diagnostics.AddError("Failed to read group", err.Error())
			return
		}
	}

	data.applyGroupState(ctx, group, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	if err := client.DeleteGroup(ctx, data.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete group", err.Error())
	}
}

func (r *GroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	user, publicKey, host, port, groupName, err := parseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	if groupName == "" {
		resp.Diagnostics.AddError("Invalid import ID", "group name must not be empty")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), groupName)...)

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

func (m *GroupResourceModel) applyGroupState(ctx context.Context, g *ssh.Group, diags *diag.Diagnostics) {
	m.Name = types.StringValue(g.Name)
	m.GID = types.Int64Value(int64(g.GID))

	if !m.Users.IsNull() {
		members := g.Members
		if members == nil {
			members = []string{}
		}
		usersVal, d := types.SetValueFrom(ctx, types.StringType, members)
		diags.Append(d...)
		m.Users = usersVal
	}
}
