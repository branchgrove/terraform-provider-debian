package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ action.Action = &AptUpgradeAction{}
var _ action.ActionWithConfigure = &AptUpgradeAction{}

func NewAptUpgradeAction() action.Action {
	return &AptUpgradeAction{}
}

type AptUpgradeAction struct {
	providerData *ProviderData
}

type AptUpgradeActionModel struct {
	DistUpgrade types.Bool      `tfsdk:"dist_upgrade"`
	Connection  ConnectionModel `tfsdk:"ssh"`
}

func (a *AptUpgradeAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apt_upgrade"
}

func (a *AptUpgradeAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_apt_upgrade` runs `apt-get upgrade -y` (or `dist-upgrade -y`) to upgrade all installed packages.",

		Attributes: map[string]schema.Attribute{
			"dist_upgrade": schema.BoolAttribute{
				MarkdownDescription: "Use `dist-upgrade` instead of `upgrade`. Defaults to `false`.",
				Optional:            true,
			},
			"ssh": actionConnectionSchema,
		},
	}
}

func (a *AptUpgradeAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Action Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T.", req.ProviderData),
		)
		return
	}

	a.providerData = pd
}

func (a *AptUpgradeAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var data AptUpgradeActionModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Connection.ApplyDefaults()

	client, err := data.Connection.GetClient(ctx, a.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	res, err := client.AptUpgrade(ctx, data.DistUpgrade.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("apt-get upgrade failed", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: string(res.Stdout),
	})
}
