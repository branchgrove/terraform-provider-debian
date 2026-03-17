package provider

import (
	"context"
	"fmt"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ConnectionModel holds the per-resource SSH connection configuration. Every
// resource must specify at least a hostname. Authentication fields (private_key,
// public_key, password) are all optional; when none are set, GetClient falls
// back to the provider-level ProviderData.PrivateKey. When public_key is set,
// GetClient looks up the corresponding private key in ProviderData.PrivateKeys.
type ConnectionModel struct {
	Hostname   types.String `tfsdk:"hostname"`
	Port       types.Int32  `tfsdk:"port"`
	User       types.String `tfsdk:"user"`
	Password   types.String `tfsdk:"password"`
	PrivateKey types.String `tfsdk:"private_key"`
	PublicKey  types.String `tfsdk:"public_key"`
	HostKey    types.String `tfsdk:"host_key"`
}

// GetClient returns an SSH client for this connection. Authentication is
// resolved with the following priority:
//  1. private_key on the resource ssh block (direct key)
//  2. public_key on the resource ssh block (looked up in ProviderData.PrivateKeys)
//  3. password on the resource ssh block
//  4. ProviderData.PrivateKey (provider-level default key)
//
// If none of the above yields an auth method, an error is returned.
func (c *ConnectionModel) GetClient(ctx context.Context, pd *ProviderData) (*ssh.Client, error) {
	auth, err := c.resolveAuth(pd)
	if err != nil {
		return nil, err
	}

	var hostKey string
	if !c.HostKey.IsNull() {
		hostKey = c.HostKey.ValueString()
	}

	return pd.SSHManager.GetClient(
		ctx,
		c.Hostname.ValueString(),
		int(c.Port.ValueInt32()),
		c.User.ValueString(),
		auth,
		hostKey,
	)
}

// resolveAuth determines the SSH auth method from the resource's connection
// fields, falling back to the provider's key ring and default private key.
func (c *ConnectionModel) resolveAuth(pd *ProviderData) (ssh.AuthMethod, error) {
	if !c.PrivateKey.IsNull() && c.PrivateKey.ValueString() != "" {
		return ssh.PrivateKeyAuth(c.PrivateKey.ValueString())
	}

	if !c.PublicKey.IsNull() && c.PublicKey.ValueString() != "" {
		pub := c.PublicKey.ValueString()
		priv, ok := pd.PrivateKeys[pub]
		if !ok {
			return nil, fmt.Errorf("public key %q not found in provider private_keys", pub)
		}
		return ssh.PrivateKeyAuth(priv)
	}

	if !c.Password.IsNull() && c.Password.ValueString() != "" {
		return ssh.PasswordAuth(c.Password.ValueString()), nil
	}

	if pd.PrivateKey != "" {
		return ssh.PrivateKeyAuth(pd.PrivateKey)
	}

	return nil, fmt.Errorf("no authentication method available: set private_key, public_key, or password on the resource ssh block, or set private_key on the provider")
}

var connectionSchema = schema.SingleNestedAttribute{
	MarkdownDescription: "SSH connection configuration. Authentication can be specified here or inherited from the provider's `private_key` / `private_keys`.",
	Required:            true,
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
			MarkdownDescription: "Password to authenticate the user on the remote host. Conflicts with `private_key` and `public_key`.",
			Sensitive:           true,
			Optional:            true,
			Validators: []validator.String{
				stringvalidator.ConflictsWith(
					path.MatchRelative().AtParent().AtName("private_key"),
					path.MatchRelative().AtParent().AtName("public_key"),
				),
			},
		},
		"private_key": schema.StringAttribute{
			MarkdownDescription: "Private key to authenticate with the remote host. Conflicts with `password` and `public_key`. When omitted, the provider's `private_key` is used.",
			Sensitive:           true,
			Optional:            true,
			Validators: []validator.String{
				stringvalidator.ConflictsWith(
					path.MatchRelative().AtParent().AtName("password"),
					path.MatchRelative().AtParent().AtName("public_key"),
				),
			},
		},
		"public_key": schema.StringAttribute{
			MarkdownDescription: "Public key whose corresponding private key is looked up in the provider's `private_keys` map. Conflicts with `private_key` and `password`.",
			Optional:            true,
			Validators: []validator.String{
				stringvalidator.ConflictsWith(
					path.MatchRelative().AtParent().AtName("private_key"),
					path.MatchRelative().AtParent().AtName("password"),
				),
			},
		},
		"host_key": schema.StringAttribute{
			MarkdownDescription: "Public key of the remote host in authorized_keys format. If set, the server's identity is verified against this key. If not set, all host keys are accepted (insecure).",
			Optional:            true,
		},
	},
}
