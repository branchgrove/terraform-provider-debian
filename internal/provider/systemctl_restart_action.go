package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ action.Action = &SystemctlRestartAction{}
var _ action.ActionWithConfigure = &SystemctlRestartAction{}

func NewSystemctlRestartAction() action.Action {
	return &SystemctlRestartAction{}
}

type SystemctlRestartAction struct {
	providerData *ProviderData
}

type SystemctlRestartActionModel struct {
	Unit       types.String    `tfsdk:"unit"`
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (a *SystemctlRestartAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_systemctl_restart"
}

func (a *SystemctlRestartAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_systemctl_restart` restarts a systemd service.",

		Attributes: map[string]schema.Attribute{
			"unit": schema.StringAttribute{
				MarkdownDescription: "Systemd unit name.",
				Required:            true,
			},
			"ssh": actionConnectionSchema,
		},
	}
}

func (a *SystemctlRestartAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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

func (a *SystemctlRestartAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var data SystemctlRestartActionModel

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

	env := map[string]string{"UNIT": data.Unit.ValueString()}
	_, err = client.Run(ctx, `systemctl restart "$UNIT"`, env, nil)
	if err != nil {
		resp.Diagnostics.AddError("systemctl restart failed", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("Restarted %s", data.Unit.ValueString()),
	})
}
