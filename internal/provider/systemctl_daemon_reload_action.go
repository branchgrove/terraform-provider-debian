package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
)

var _ action.Action = &SystemctlDaemonReloadAction{}
var _ action.ActionWithConfigure = &SystemctlDaemonReloadAction{}

func NewSystemctlDaemonReloadAction() action.Action {
	return &SystemctlDaemonReloadAction{}
}

type SystemctlDaemonReloadAction struct {
	providerData *ProviderData
}

type SystemctlDaemonReloadActionModel struct {
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (a *SystemctlDaemonReloadAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_systemctl_daemon_reload"
}

func (a *SystemctlDaemonReloadAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_systemctl_daemon_reload` runs `systemctl daemon-reload` to reload all systemd unit files.",

		Attributes: map[string]schema.Attribute{
			"ssh": actionConnectionSchema,
		},
	}
}

func (a *SystemctlDaemonReloadAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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

func (a *SystemctlDaemonReloadAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var data SystemctlDaemonReloadActionModel

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

	_, err = client.Run(ctx, `systemctl daemon-reload`, nil, nil)
	if err != nil {
		resp.Diagnostics.AddError("systemctl daemon-reload failed", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: "Reloaded systemd daemon",
	})
}
