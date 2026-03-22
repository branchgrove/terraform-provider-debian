package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &AptPackagesResource{}

func NewAptPackagesResource() resource.Resource {
	return &AptPackagesResource{}
}

type AptPackagesResource struct {
	providerData *ProviderData
}

type AptPackagesResourceModel struct {
	Packages          types.Map       `tfsdk:"packages"`
	Update            types.Bool      `tfsdk:"update"`
	Purge             types.Bool      `tfsdk:"purge"`
	InstalledPackages types.Map       `tfsdk:"installed_packages"`
	Connection        ConnectionModel `tfsdk:"ssh"`
}

func (r *AptPackagesResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apt_packages"
}

func (r *AptPackagesResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_apt_packages` manages a set of installed packages on the remote host.",

		Attributes: map[string]schema.Attribute{
			"packages": schema.MapAttribute{
				MarkdownDescription: "Map of package name to version constraint. Use `\"\"` or `\"*\"` for latest available.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"update": schema.BoolAttribute{
				MarkdownDescription: "Run `apt-get update` before reconciling packages. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"purge": schema.BoolAttribute{
				MarkdownDescription: "Use `apt-get purge` instead of `remove`. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"installed_packages": schema.MapAttribute{
				MarkdownDescription: "Actual installed versions, keyed by package name.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"ssh": connectionSchema,
		},
	}
}

func (r *AptPackagesResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AptPackagesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AptPackagesResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	var packages map[string]string
	resp.Diagnostics.Append(data.Packages.ElementsAs(ctx, &packages, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := client.AptInstall(ctx, packages, data.Update.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Failed to install packages", err.Error())
		return
	}

	r.readInstalled(ctx, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AptPackagesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AptPackagesResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readInstalled(ctx, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AptPackagesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AptPackagesResourceModel
	var state AptPackagesResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := plan.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	var wantPkgs map[string]string
	resp.Diagnostics.Append(plan.Packages.ElementsAs(ctx, &wantPkgs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var hadPkgs map[string]string
	resp.Diagnostics.Append(state.Packages.ElementsAs(ctx, &hadPkgs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Packages removed from the map need to be uninstalled
	var toRemove []string
	for name := range hadPkgs {
		if _, ok := wantPkgs[name]; !ok {
			toRemove = append(toRemove, name)
		}
	}
	if len(toRemove) > 0 {
		if err := client.AptRemove(ctx, toRemove, plan.Purge.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Failed to remove packages", err.Error())
			return
		}
	}

	// Install/upgrade packages that are new or have a changed version constraint
	if err := client.AptInstall(ctx, wantPkgs, plan.Update.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Failed to install packages", err.Error())
		return
	}

	r.readInstalled(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AptPackagesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AptPackagesResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	var packages map[string]string
	resp.Diagnostics.Append(data.Packages.ElementsAs(ctx, &packages, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}

	if err := client.AptRemove(ctx, names, data.Purge.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Failed to remove packages", err.Error())
	}
}

func (r *AptPackagesResource) readInstalled(ctx context.Context, data *AptPackagesResourceModel, diags *diag.Diagnostics) {
	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		diags.AddError("Failed to get SSH client", err.Error())
		return
	}

	var packages map[string]string
	diags.Append(data.Packages.ElementsAs(ctx, &packages, false)...)
	if diags.HasError() {
		return
	}

	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}

	installed, err := client.GetInstalledPackages(ctx, names)
	if err != nil {
		diags.AddError("Failed to read installed packages", err.Error())
		return
	}

	installedVal, d := types.MapValueFrom(ctx, types.StringType, installed)
	diags.Append(d...)
	data.InstalledPackages = installedVal
}
