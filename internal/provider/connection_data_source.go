package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// dataSourceConnectionSchema mirrors connectionSchema for data sources. The
// datasource/schema package uses distinct types from resource/schema so we
// cannot share a single variable.
var dataSourceConnectionSchema = schema.SingleNestedAttribute{
	MarkdownDescription: "SSH connection configuration. Specify authentication here or inherit it from the provider's `private_key` / `private_keys`.",
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
		},
		"user": schema.StringAttribute{
			MarkdownDescription: "User on the remote host, the default is `root`.",
			Optional:            true,
			Computed:            true,
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
