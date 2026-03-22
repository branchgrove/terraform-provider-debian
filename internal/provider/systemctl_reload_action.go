package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ action.Action = &SystemctlReloadAction{}
var _ action.ActionWithConfigure = &SystemctlReloadAction{}

func NewSystemctlReloadAction() action.Action {
	return &SystemctlReloadAction{}
}

type SystemctlReloadAction struct {
	providerData *ProviderData
}

type SystemctlReloadActionModel struct {
	Unit       types.String    `tfsdk:"unit"`
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (a *SystemctlReloadAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_systemctl_reload"
}

func (a *SystemctlReloadAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_systemctl_reload` reloads a systemd service configuration.",

		Attributes: map[string]schema.Attribute{
			"unit": schema.StringAttribute{
				MarkdownDescription: "Systemd unit name.",
				Required:            true,
			},
			"ssh": actionConnectionSchema,
		},
	}
}

func (a *SystemctlReloadAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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

func (a *SystemctlReloadAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var data SystemctlReloadActionModel

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
	_, err = client.Run(ctx, `systemctl reload "$UNIT"`, env, nil)
	if err != nil {
		resp.Diagnostics.AddError("systemctl reload failed", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("Reloaded %s", data.Unit.ValueString()),
	})
}
