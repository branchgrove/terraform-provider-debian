package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ action.Action = &CommandAction{}
var _ action.ActionWithConfigure = &CommandAction{}

func NewCommandAction() action.Action {
	return &CommandAction{}
}

type CommandAction struct {
	providerData *ProviderData
}

type CommandActionModel struct {
	Command    types.String    `tfsdk:"command"`
	Env        types.Map       `tfsdk:"env"`
	Stdin      types.String    `tfsdk:"stdin"`
	AllowError types.Bool      `tfsdk:"allow_error"`
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (a *CommandAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_command"
}

func (a *CommandAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_command` runs an arbitrary command on the remote host and reports stdout, stderr, and exit code via progress messages.",

		Attributes: map[string]schema.Attribute{
			"command": schema.StringAttribute{
				MarkdownDescription: "Command to execute.",
				Required:            true,
			},
			"env": schema.MapAttribute{
				MarkdownDescription: "Environment variables to set for the command.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"stdin": schema.StringAttribute{
				MarkdownDescription: "Standard input to pass to the command.",
				Optional:            true,
			},
			"allow_error": schema.BoolAttribute{
				MarkdownDescription: "If `true`, non-zero exit codes are not treated as errors. Defaults to `false`.",
				Optional:            true,
			},
			"ssh": actionConnectionSchema,
		},
	}
}

func (a *CommandAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Action Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	a.providerData = pd
}

func (a *CommandAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var data CommandActionModel

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

	var env map[string]string
	if !data.Env.IsNull() && !data.Env.IsUnknown() {
		env = make(map[string]string)
		resp.Diagnostics.Append(data.Env.ElementsAs(ctx, &env, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var stdin io.Reader
	if !data.Stdin.IsNull() && !data.Stdin.IsUnknown() {
		stdin = strings.NewReader(data.Stdin.ValueString())
	}

	res, err := client.Run(ctx, data.Command.ValueString(), env, stdin)
	if err != nil {
		var runErr *ssh.RunError
		if errors.As(err, &runErr) && data.AllowError.ValueBool() {
			resp.SendProgress(action.InvokeProgressEvent{
				Message: fmt.Sprintf("stdout: %s", string(runErr.Stdout)),
			})
			resp.SendProgress(action.InvokeProgressEvent{
				Message: fmt.Sprintf("stderr: %s", string(runErr.Stderr)),
			})
			resp.SendProgress(action.InvokeProgressEvent{
				Message: fmt.Sprintf("exit_code: %d", runErr.ExitCode),
			})
			return
		}
		resp.Diagnostics.AddError("Command failed", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("stdout: %s", string(res.Stdout)),
	})
	if len(res.Stderr) > 0 {
		resp.SendProgress(action.InvokeProgressEvent{
			Message: fmt.Sprintf("stderr: %s", string(res.Stderr)),
		})
	}
	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("exit_code: %d", res.ExitCode),
	})
}
