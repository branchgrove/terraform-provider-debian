package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &CommandDataSource{}
var _ datasource.DataSourceWithConfigure = &CommandDataSource{}

func NewCommandDataSource() datasource.DataSource {
	return &CommandDataSource{}
}

type CommandDataSource struct {
	providerData *ProviderData
}

type CommandDataSourceModel struct {
	Command    types.String `tfsdk:"command"`
	Env        types.Map    `tfsdk:"env"`
	Stdin      types.String `tfsdk:"stdin"`
	AllowError types.Bool   `tfsdk:"allow_error"`
	Stdout     types.String `tfsdk:"stdout"`
	Stderr     types.String `tfsdk:"stderr"`
	ExitCode   types.Int64  `tfsdk:"exit_code"`
	Connection ConnectionModel `tfsdk:"ssh"`
}

func (d *CommandDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_command"
}

func (d *CommandDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_command` runs an arbitrary command on the remote host and returns its output. The command is executed on every plan/apply refresh.",

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
			"stdout": schema.StringAttribute{
				MarkdownDescription: "Standard output of the command.",
				Computed:            true,
			},
			"stderr": schema.StringAttribute{
				MarkdownDescription: "Standard error of the command.",
				Computed:            true,
			},
			"exit_code": schema.Int64Attribute{
				MarkdownDescription: "Exit code of the command.",
				Computed:            true,
			},
			"ssh": dataSourceConnectionSchema,
		},
	}
}

func (d *CommandDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.providerData = pd
}

func (d *CommandDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CommandDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Connection.ApplyDefaults()

	client, err := data.Connection.GetClient(ctx, d.providerData)
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
			data.Stdout = types.StringValue(string(runErr.Stdout))
			data.Stderr = types.StringValue(string(runErr.Stderr))
			data.ExitCode = types.Int64Value(int64(runErr.ExitCode))
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
		resp.Diagnostics.AddError("Command failed", err.Error())
		return
	}

	data.Stdout = types.StringValue(string(res.Stdout))
	data.Stderr = types.StringValue(string(res.Stderr))
	data.ExitCode = types.Int64Value(int64(res.ExitCode))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
