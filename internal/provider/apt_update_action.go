package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
)

var _ action.Action = &AptUpdateAction{}
var _ action.ActionWithConfigure = &AptUpdateAction{}

func NewAptUpdateAction() action.Action {
	return &AptUpdateAction{}
}

type AptUpdateAction struct {
	providerData *ProviderData
}

type AptUpdateActionModel struct {
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (a *AptUpdateAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apt_update"
}

func (a *AptUpdateAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_apt_update` runs `apt-get update` to refresh the package index.",

		Attributes: map[string]schema.Attribute{
			"ssh": actionConnectionSchema,
		},
	}
}

func (a *AptUpdateAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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

func (a *AptUpdateAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var data AptUpdateActionModel

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

	res, err := client.AptUpdate(ctx)
	if err != nil {
		resp.Diagnostics.AddError("apt-get update failed", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: string(res.Stdout),
	})
}
