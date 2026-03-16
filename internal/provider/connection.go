package provider

import (
	"context"
	"fmt"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ConnectionModel struct {
	Hostname   types.String `tfsdk:"hostname"`
	Port       types.Int32  `tfsdk:"port"`
	User       types.String `tfsdk:"user"`
	Password   types.String `tfsdk:"password"`
	PrivateKey types.String `tfsdk:"private_key"`
	HostKey    types.String `tfsdk:"host_key"`
}

func (c *ConnectionModel) AuthMethod() (ssh.AuthMethod, error) {
	if c.Password.IsNull() && c.PrivateKey.IsNull() {
		return nil, fmt.Errorf("password or private key is required")
	}

	if !c.Password.IsNull() {
		return ssh.PasswordAuth(c.Password.ValueString()), nil
	}

	auth, err := ssh.PrivateKeyAuth(c.PrivateKey.ValueString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return auth, nil
}

func (c *ConnectionModel) GetClient(ctx context.Context, sshManager *ssh.Manager) (*ssh.Client, error) {
	authMethod, err := c.AuthMethod()
	if err != nil {
		return nil, err
	}

	var hostKey string
	if !c.HostKey.IsNull() {
		hostKey = c.HostKey.ValueString()
	}

	return sshManager.GetClient(
		ctx,
		c.Hostname.ValueString(),
		int(c.Port.ValueInt32()),
		c.User.ValueString(),
		authMethod,
		hostKey,
	)
}

var connectionSchema = schema.SingleNestedAttribute{
	MarkdownDescription: "Configuration used to connect with ssh.",
	// TODO: set to optional and allow provider default
	Required: true,
	Attributes: map[string]schema.Attribute{
		"hostname": schema.StringAttribute{
			MarkdownDescription: "The remote host to connect to.",
			Required:            true,
		},
		"port": schema.Int32Attribute{
			MarkdownDescription: "Port that the remote host uses for ssh.",
			Optional:            true,
			Computed:            true,
			Default:             int32default.StaticInt32(22),
		},
		"user": schema.StringAttribute{
			MarkdownDescription: "User on the remote host, the default is `root`.",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("root"),
		},
		"password": schema.StringAttribute{
			MarkdownDescription: "Password to authenticate the user on the remote host. Prefer to use ssh keys.",
			Sensitive:           true,
			Optional:            true,
		},
		"private_key": schema.StringAttribute{
			MarkdownDescription: "Private key to authenticate the user on the remote host. Prefer to use ssh keys.",
			Sensitive:           true,
			Optional:            true,
		},
		"host_key": schema.StringAttribute{
			MarkdownDescription: "Public key of the remote host in authorized_keys format. If set, the server's identity is verified against this key. If not set, all host keys are accepted (insecure).",
			Optional:            true,
		},
	},
}
