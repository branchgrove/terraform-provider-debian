package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &GroupMemberResource{}
var _ resource.ResourceWithImportState = &GroupMemberResource{}

func NewGroupMemberResource() resource.Resource {
	return &GroupMemberResource{}
}

type GroupMemberResource struct {
	providerData *ProviderData
}

type GroupMemberResourceModel struct {
	User       types.String    `tfsdk:"user"`
	Group      types.String    `tfsdk:"group"`
	Overwrite  types.Bool      `tfsdk:"overwrite"`
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (r *GroupMemberResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_member"
}

func (r *GroupMemberResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_group_member` manages a single user's membership in a single group. Use this instead of `debian_group.users` or `debian_user.groups` when multiple modules need to independently manage memberships.",

		Attributes: map[string]schema.Attribute{
			"user": schema.StringAttribute{
				MarkdownDescription: "Username to add to the group. Changing this forces recreation.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"group": schema.StringAttribute{
				MarkdownDescription: "Group to add the user to. Changing this forces recreation.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"overwrite": schema.BoolAttribute{
				MarkdownDescription: "Continue instead of failing if the membership already exists. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"ssh": connectionSchema,
		},
	}
}

func (r *GroupMemberResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GroupMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupMemberResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	group, err := client.GetGroup(ctx, data.Group.ValueString())
	if err == nil {
		if slices.Contains(group.Members, data.User.ValueString()) {
			if !data.Overwrite.ValueBool() {
				resp.Diagnostics.AddError("Resource already exists", "The group membership already exists and overwrite is false")
				return
			}
		}
	} else if !errors.Is(err, ssh.ErrNotFound) {
		resp.Diagnostics.AddError("Failed to check if group exists", err.Error())
		return
	}

	if err := client.AddGroupMember(ctx, data.Group.ValueString(), data.User.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to add group member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupMemberResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	group, err := client.GetGroup(ctx, data.Group.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if !slices.Contains(group.Members, data.User.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GroupMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Both fields are ForceNew, so Update should never be called.
	resp.Diagnostics.AddError("Unexpected update", "group_member does not support updates; both user and group require replacement")
}

func (r *GroupMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupMemberResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	if err := client.RemoveGroupMember(ctx, data.Group.ValueString(), data.User.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to remove group member", err.Error())
	}
}

func (r *GroupMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	user, publicKey, host, port, resourceID, err := parseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	parts := strings.SplitN(resourceID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected id=username:groupname, got %q", resourceID))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group"), parts[1])...)

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
