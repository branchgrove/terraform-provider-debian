package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SystemdServiceResource{}
var _ resource.ResourceWithImportState = &SystemdServiceResource{}

func NewSystemdServiceResource() resource.Resource {
	return &SystemdServiceResource{}
}

type SystemdServiceResource struct {
	providerData *ProviderData
}

// --------------------------------------------------------------------
// Model types
// --------------------------------------------------------------------

type SystemdServiceResourceModel struct {
	Name          types.String                `tfsdk:"name"`
	Enabled       types.Bool                  `tfsdk:"enabled"`
	Active        types.Bool                  `tfsdk:"active"`
	ActiveTimeout types.Int64                 `tfsdk:"active_timeout"`
	Unit          *SystemdUnitSectionModel    `tfsdk:"unit"`
	Service       *SystemdServiceSectionModel `tfsdk:"service"`
	Install       *SystemdInstallSectionModel `tfsdk:"install"`
	Overwrite     types.Bool                  `tfsdk:"overwrite"`
	Connection    ConnectionModel             `tfsdk:"ssh"`
}

func (m *SystemdServiceResourceModel) ownsUnitFile() bool {
	return m.Unit != nil || m.Service != nil || m.Install != nil
}

type SystemdUnitSectionModel struct {
	Description   types.String       `tfsdk:"description"`
	Documentation types.List         `tfsdk:"documentation"`
	After         types.List         `tfsdk:"after"`
	Before        types.List         `tfsdk:"before"`
	Requires      types.List         `tfsdk:"requires"`
	Wants         types.List         `tfsdk:"wants"`
	BindsTo       types.List         `tfsdk:"binds_to"`
	PartOf        types.List         `tfsdk:"part_of"`
	Conflicts     types.List         `tfsdk:"conflicts"`
	Condition     *SystemdCheckModel `tfsdk:"condition"`
	Assert        *SystemdCheckModel `tfsdk:"assert"`
	Extra         types.Map          `tfsdk:"extra"`
}

type SystemdCheckModel struct {
	PathExists         types.List   `tfsdk:"path_exists"`
	PathIsDirectory    types.List   `tfsdk:"path_is_directory"`
	PathIsSymbolicLink types.List   `tfsdk:"path_is_symbolic_link"`
	FileNotEmpty       types.List   `tfsdk:"file_not_empty"`
	DirectoryNotEmpty  types.List   `tfsdk:"directory_not_empty"`
	User               types.String `tfsdk:"user"`
	Group              types.String `tfsdk:"group"`
	Host               types.String `tfsdk:"host"`
	Virtualization     types.String `tfsdk:"virtualization"`
	Security           types.String `tfsdk:"security"`
}

type SystemdServiceSectionModel struct {
	Type             types.String `tfsdk:"type"`
	ExecStart        types.String `tfsdk:"exec_start"`
	ExecStartPre     types.List   `tfsdk:"exec_start_pre"`
	ExecStartPost    types.List   `tfsdk:"exec_start_post"`
	ExecStop         types.String `tfsdk:"exec_stop"`
	ExecStopPost     types.List   `tfsdk:"exec_stop_post"`
	ExecReload       types.String `tfsdk:"exec_reload"`
	Restart          types.String `tfsdk:"restart"`
	RestartSec       types.String `tfsdk:"restart_sec"`
	TimeoutStartSec  types.String `tfsdk:"timeout_start_sec"`
	TimeoutStopSec   types.String `tfsdk:"timeout_stop_sec"`
	User             types.String `tfsdk:"user"`
	Group            types.String `tfsdk:"group"`
	WorkingDirectory types.String `tfsdk:"working_directory"`
	Environment      types.Map    `tfsdk:"environment"`
	EnvironmentFile  types.String `tfsdk:"environment_file"`
	StandardOutput   types.String `tfsdk:"standard_output"`
	StandardError    types.String `tfsdk:"standard_error"`
	RemainAfterExit  types.Bool   `tfsdk:"remain_after_exit"`
	Extra            types.Map    `tfsdk:"extra"`
}

type SystemdInstallSectionModel struct {
	WantedBy   types.List `tfsdk:"wanted_by"`
	RequiredBy types.List `tfsdk:"required_by"`
	Alias      types.List `tfsdk:"alias"`
	Extra      types.Map  `tfsdk:"extra"`
}

// --------------------------------------------------------------------
// Resource interface
// --------------------------------------------------------------------

func (r *SystemdServiceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_systemd_service"
}

func (r *SystemdServiceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	checkAttrs := map[string]schema.Attribute{
		"path_exists": schema.ListAttribute{
			MarkdownDescription: "Paths that must exist. Prefix with `!` to negate.",
			Optional:            true,
			ElementType:         types.StringType,
		},
		"path_is_directory": schema.ListAttribute{
			MarkdownDescription: "Paths that must be directories.",
			Optional:            true,
			ElementType:         types.StringType,
		},
		"path_is_symbolic_link": schema.ListAttribute{
			MarkdownDescription: "Paths that must be symbolic links.",
			Optional:            true,
			ElementType:         types.StringType,
		},
		"file_not_empty": schema.ListAttribute{
			MarkdownDescription: "Paths that must be non-empty files.",
			Optional:            true,
			ElementType:         types.StringType,
		},
		"directory_not_empty": schema.ListAttribute{
			MarkdownDescription: "Paths that must be non-empty directories.",
			Optional:            true,
			ElementType:         types.StringType,
		},
		"user": schema.StringAttribute{
			MarkdownDescription: "User (name or UID) that systemd must be running as.",
			Optional:            true,
		},
		"group": schema.StringAttribute{
			MarkdownDescription: "Group (name or GID) that systemd must be running as.",
			Optional:            true,
		},
		"host": schema.StringAttribute{
			MarkdownDescription: "Hostname or machine ID that must match.",
			Optional:            true,
		},
		"virtualization": schema.StringAttribute{
			MarkdownDescription: "Virtualization type check (for example, `vm`, `container`, `!container`).",
			Optional:            true,
		},
		"security": schema.StringAttribute{
			MarkdownDescription: "Security framework check (for example, `selinux`, `apparmor`).",
			Optional:            true,
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "`debian_systemd_service` manages a systemd `.service` unit on the remote host. Supports both package-installed services (state management only) and custom services (unit file + state management).",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Service unit name (without `.service` extension).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the service is enabled (`systemctl enable` / `disable`).",
				Required:            true,
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the service is running (`systemctl start` / `stop`).",
				Required:            true,
			},
			"active_timeout": schema.Int64Attribute{
				MarkdownDescription: "Timeout in seconds to wait for the service to become active. Defaults to 15.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(15),
			},
			"unit": schema.SingleNestedAttribute{
				MarkdownDescription: "`[Unit]` section. Optional — omit for package-installed services.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"description": schema.StringAttribute{
						MarkdownDescription: "`Description=` — human-readable description of the unit.",
						Required:            true,
					},
					"documentation": schema.ListAttribute{
						MarkdownDescription: "`Documentation=` — URIs referencing documentation.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"after": schema.ListAttribute{
						MarkdownDescription: "`After=` — units to order after.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"before": schema.ListAttribute{
						MarkdownDescription: "`Before=` — units to order before.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"requires": schema.ListAttribute{
						MarkdownDescription: "`Requires=` — hard dependencies.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"wants": schema.ListAttribute{
						MarkdownDescription: "`Wants=` — soft dependencies.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"binds_to": schema.ListAttribute{
						MarkdownDescription: "`BindsTo=` — stronger than Requires; stop if dependency stops.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"part_of": schema.ListAttribute{
						MarkdownDescription: "`PartOf=` — stop/restart when listed units stop/restart.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"conflicts": schema.ListAttribute{
						MarkdownDescription: "`Conflicts=` — units that cannot run simultaneously.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"condition": schema.SingleNestedAttribute{
						MarkdownDescription: "`Condition*=` directives. Failing conditions cause the unit to skip silently.",
						Optional:            true,
						Attributes:          checkAttrs,
					},
					"assert": schema.SingleNestedAttribute{
						MarkdownDescription: "`Assert*=` directives. Failing asserts cause the unit to fail with an error.",
						Optional:            true,
						Attributes:          checkAttrs,
					},
					"extra": schema.MapAttribute{
						MarkdownDescription: "Additional `[Unit]` directives not covered by named attributes. Keys must use exact systemd directive names.",
						Optional:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"service": schema.SingleNestedAttribute{
				MarkdownDescription: "`[Service]` section. Optional — omit for package-installed services.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "`Type=` — `simple`, `exec`, `forking`, `oneshot`, `notify`, `idle`.",
						Optional:            true,
					},
					"exec_start": schema.StringAttribute{
						MarkdownDescription: "`ExecStart=` — command to run.",
						Required:            true,
					},
					"exec_start_pre": schema.ListAttribute{
						MarkdownDescription: "`ExecStartPre=` — commands to run before ExecStart.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"exec_start_post": schema.ListAttribute{
						MarkdownDescription: "`ExecStartPost=` — commands to run after ExecStart.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"exec_stop": schema.StringAttribute{
						MarkdownDescription: "`ExecStop=` — command to stop the service.",
						Optional:            true,
					},
					"exec_stop_post": schema.ListAttribute{
						MarkdownDescription: "`ExecStopPost=` — commands to run after the service stops.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"exec_reload": schema.StringAttribute{
						MarkdownDescription: "`ExecReload=` — command to reload configuration.",
						Optional:            true,
					},
					"restart": schema.StringAttribute{
						MarkdownDescription: "`Restart=` — `no`, `always`, `on-success`, `on-failure`, `on-abnormal`.",
						Optional:            true,
					},
					"restart_sec": schema.StringAttribute{
						MarkdownDescription: "`RestartSec=` — delay before restart.",
						Optional:            true,
					},
					"timeout_start_sec": schema.StringAttribute{
						MarkdownDescription: "`TimeoutStartSec=` — startup timeout.",
						Optional:            true,
					},
					"timeout_stop_sec": schema.StringAttribute{
						MarkdownDescription: "`TimeoutStopSec=` — stop timeout.",
						Optional:            true,
					},
					"user": schema.StringAttribute{
						MarkdownDescription: "`User=` — UNIX user to run as.",
						Optional:            true,
					},
					"group": schema.StringAttribute{
						MarkdownDescription: "`Group=` — UNIX group to run as.",
						Optional:            true,
					},
					"working_directory": schema.StringAttribute{
						MarkdownDescription: "`WorkingDirectory=` — working directory for the process.",
						Optional:            true,
					},
					"environment": schema.MapAttribute{
						MarkdownDescription: "`Environment=` — env vars. Each entry becomes `Environment=\"K=V\"`.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"environment_file": schema.StringAttribute{
						MarkdownDescription: "`EnvironmentFile=` — path to an env file.",
						Optional:            true,
					},
					"standard_output": schema.StringAttribute{
						MarkdownDescription: "`StandardOutput=` — Destination for standard output.",
						Optional:            true,
					},
					"standard_error": schema.StringAttribute{
						MarkdownDescription: "`StandardError=` — Destination for standard error.",
						Optional:            true,
					},
					"remain_after_exit": schema.BoolAttribute{
						MarkdownDescription: "`RemainAfterExit=` — stay active after main process exits.",
						Optional:            true,
					},
					"extra": schema.MapAttribute{
						MarkdownDescription: "Additional `[Service]` directives. Keys must use exact systemd directive names.",
						Optional:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"install": schema.SingleNestedAttribute{
				MarkdownDescription: "`[Install]` section. Optional — omit for package-installed services.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"wanted_by": schema.ListAttribute{
						MarkdownDescription: "`WantedBy=` — targets that pull in this service when enabled.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"required_by": schema.ListAttribute{
						MarkdownDescription: "`RequiredBy=` — targets that require this service when enabled.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"alias": schema.ListAttribute{
						MarkdownDescription: "`Alias=` — alternative names for the unit.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"extra": schema.MapAttribute{
						MarkdownDescription: "Additional `[Install]` directives. Keys must use exact systemd directive names.",
						Optional:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"overwrite": schema.BoolAttribute{
				MarkdownDescription: "Overwrite the resource if it already exists. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"ssh": connectionSchema,
		},
	}
}

func (r *SystemdServiceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.providerData = pd
}

// --------------------------------------------------------------------
// CRUD
// --------------------------------------------------------------------

func (r *SystemdServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SystemdServiceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	name := data.Name.ValueString()

	exists, err := client.ServiceUnitExists(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to check if service unit exists", err.Error())
		return
	}
	if exists {
		if !data.Overwrite.ValueBool() {
			resp.Diagnostics.AddError("Resource already exists", "The service unit already exists and overwrite is false")
			return
		}
	}

	if data.ownsUnitFile() {
		unit := data.toServiceUnit(ctx, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		if err := client.WriteServiceUnit(ctx, name, unit); err != nil {
			resp.Diagnostics.AddError("Failed to write unit file", err.Error())
			return
		}

		if err := client.DaemonReload(ctx); err != nil {
			resp.Diagnostics.AddError("Failed to daemon-reload", err.Error())
			return
		}
	}

	if data.Enabled.ValueBool() {
		if err := client.EnableService(ctx, name); err != nil {
			resp.Diagnostics.AddError("Failed to enable service", err.Error())
			return
		}
	}

	if data.Active.ValueBool() {
		if err := client.StartService(ctx, name); err != nil {
			resp.Diagnostics.AddError("Failed to start service", err.Error())
			return
		}
		timeout := time.Duration(data.ActiveTimeout.ValueInt64()) * time.Second
		if err := client.WaitServiceActive(ctx, name, timeout); err != nil {
			resp.Diagnostics.AddError("Failed to wait for service to become active", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SystemdServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SystemdServiceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	name := data.Name.ValueString()
	isImport := data.Enabled.IsNull()

	if data.ownsUnitFile() {
		unit, err := client.ReadServiceUnit(ctx, name)
		if err != nil {
			if errors.Is(err, ssh.ErrNotFound) {
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError("Failed to read unit file", err.Error())
			return
		}

		data.applyServiceUnit(ctx, unit, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	} else if isImport {
		exists, err := client.ServiceUnitExists(ctx, name)
		if err != nil {
			resp.Diagnostics.AddError("Failed to check unit file", err.Error())
			return
		}
		if exists {
			unit, err := client.ReadServiceUnit(ctx, name)
			if err != nil {
				resp.Diagnostics.AddError("Failed to read unit file", err.Error())
				return
			}
			data.applyServiceUnit(ctx, unit, &resp.Diagnostics)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	state, err := client.GetServiceState(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get service state", err.Error())
		return
	}

	data.Enabled = types.BoolValue(state.Enabled)
	data.Active = types.BoolValue(state.Active)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SystemdServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SystemdServiceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := plan.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	name := plan.Name.ValueString()
	unitFileChanged := false

	if plan.ownsUnitFile() {
		planUnit := plan.toServiceUnit(ctx, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		planContent := planUnit.Serialize()
		stateContent := ""
		if state.ownsUnitFile() {
			stateUnit := state.toServiceUnit(ctx, &resp.Diagnostics)
			if resp.Diagnostics.HasError() {
				return
			}
			stateContent = stateUnit.Serialize()
		}

		unitFileChanged = planContent != stateContent

		if unitFileChanged {
			if err := client.WriteServiceUnit(ctx, name, planUnit); err != nil {
				resp.Diagnostics.AddError("Failed to write unit file", err.Error())
				return
			}

			if err := client.DaemonReload(ctx); err != nil {
				resp.Diagnostics.AddError("Failed to daemon-reload", err.Error())
				return
			}
		}
	} else if state.ownsUnitFile() {
		if err := client.DeleteServiceUnit(ctx, name); err != nil {
			resp.Diagnostics.AddError("Failed to delete unit file", err.Error())
			return
		}
		if err := client.DaemonReload(ctx); err != nil {
			resp.Diagnostics.AddError("Failed to daemon-reload", err.Error())
			return
		}
		unitFileChanged = true
	}

	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		if plan.Enabled.ValueBool() {
			if err := client.EnableService(ctx, name); err != nil {
				resp.Diagnostics.AddError("Failed to enable service", err.Error())
				return
			}
		} else {
			if err := client.DisableService(ctx, name); err != nil {
				resp.Diagnostics.AddError("Failed to disable service", err.Error())
				return
			}
		}
	}

	if plan.Active.ValueBool() != state.Active.ValueBool() {
		if plan.Active.ValueBool() {
			if err := client.StartService(ctx, name); err != nil {
				resp.Diagnostics.AddError("Failed to start service", err.Error())
				return
			}
			timeout := time.Duration(plan.ActiveTimeout.ValueInt64()) * time.Second
			if err := client.WaitServiceActive(ctx, name, timeout); err != nil {
				resp.Diagnostics.AddError("Failed to wait for service to become active", err.Error())
				return
			}
		} else {
			if err := client.StopService(ctx, name); err != nil {
				resp.Diagnostics.AddError("Failed to stop service", err.Error())
				return
			}
		}
	} else if unitFileChanged && plan.Active.ValueBool() {
		if err := client.RestartService(ctx, name); err != nil {
			resp.Diagnostics.AddError("Failed to restart service", err.Error())
			return
		}
		timeout := time.Duration(plan.ActiveTimeout.ValueInt64()) * time.Second
		if err := client.WaitServiceActive(ctx, name, timeout); err != nil {
			resp.Diagnostics.AddError("Failed to wait for service to become active after restart", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SystemdServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SystemdServiceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, err := data.Connection.GetClient(ctx, r.providerData)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get SSH client", err.Error())
		return
	}

	name := data.Name.ValueString()

	if data.Active.ValueBool() {
		if err := client.StopService(ctx, name); err != nil {
			resp.Diagnostics.AddError("Failed to stop service", err.Error())
			return
		}
	}

	if data.Enabled.ValueBool() {
		if err := client.DisableService(ctx, name); err != nil {
			resp.Diagnostics.AddError("Failed to disable service", err.Error())
			return
		}
	}

	if data.ownsUnitFile() {
		if err := client.DeleteServiceUnit(ctx, name); err != nil {
			resp.Diagnostics.AddError("Failed to delete unit file", err.Error())
			return
		}
		if err := client.DaemonReload(ctx); err != nil {
			resp.Diagnostics.AddError("Failed to daemon-reload", err.Error())
			return
		}
	}
}

func (r *SystemdServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	user, publicKey, host, port, serviceName, err := parseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	if serviceName == "" {
		resp.Diagnostics.AddError("Invalid import ID", "service name must not be empty")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), serviceName)...)

	conn := ConnectionModel{
		Hostname: types.StringValue(host),
		Port:     types.Int32Value(int32(port)),
		User:     types.StringValue(user),
	}
	if publicKey != "" {
		conn.PublicKey = types.StringValue(publicKey)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ssh"), conn)...)
}

// --------------------------------------------------------------------
// Terraform model → SSH model
// --------------------------------------------------------------------

// toServiceUnit converts the Terraform resource model to an ssh.ServiceUnit,
// bridging Terraform's tri-state types to plain Go values.
func (m *SystemdServiceResourceModel) toServiceUnit(ctx context.Context, diags *diag.Diagnostics) *ssh.ServiceUnit {
	u := &ssh.ServiceUnit{}

	if m.Unit != nil {
		u.Unit = m.Unit.toUnitSection(ctx, diags)
	}
	if m.Service != nil {
		u.Service = m.Service.toServiceSection(ctx, diags)
	}
	if m.Install != nil {
		u.Install = m.Install.toInstallSection(ctx, diags)
	}

	return u
}

func (m *SystemdUnitSectionModel) toUnitSection(ctx context.Context, diags *diag.Diagnostics) *ssh.UnitSection {
	s := &ssh.UnitSection{
		Description:   m.Description.ValueString(),
		Documentation: toStringSlice(ctx, m.Documentation, diags),
		After:         toStringSlice(ctx, m.After, diags),
		Before:        toStringSlice(ctx, m.Before, diags),
		Requires:      toStringSlice(ctx, m.Requires, diags),
		Wants:         toStringSlice(ctx, m.Wants, diags),
		BindsTo:       toStringSlice(ctx, m.BindsTo, diags),
		PartOf:        toStringSlice(ctx, m.PartOf, diags),
		Conflicts:     toStringSlice(ctx, m.Conflicts, diags),
		Extra:         toStringMap(ctx, m.Extra, diags),
	}

	if m.Condition != nil {
		s.Condition = m.Condition.toCheckDirectives(ctx, diags)
	}
	if m.Assert != nil {
		s.Assert = m.Assert.toCheckDirectives(ctx, diags)
	}

	return s
}

func (m *SystemdCheckModel) toCheckDirectives(ctx context.Context, diags *diag.Diagnostics) *ssh.CheckDirectives {
	return &ssh.CheckDirectives{
		PathExists:         toStringSlice(ctx, m.PathExists, diags),
		PathIsDirectory:    toStringSlice(ctx, m.PathIsDirectory, diags),
		PathIsSymbolicLink: toStringSlice(ctx, m.PathIsSymbolicLink, diags),
		FileNotEmpty:       toStringSlice(ctx, m.FileNotEmpty, diags),
		DirectoryNotEmpty:  toStringSlice(ctx, m.DirectoryNotEmpty, diags),
		User:               m.User.ValueString(),
		Group:              m.Group.ValueString(),
		Host:               m.Host.ValueString(),
		Virtualization:     m.Virtualization.ValueString(),
		Security:           m.Security.ValueString(),
	}
}

func (m *SystemdServiceSectionModel) toServiceSection(ctx context.Context, diags *diag.Diagnostics) *ssh.ServiceSection {
	s := &ssh.ServiceSection{
		Type:             m.Type.ValueString(),
		ExecStart:        m.ExecStart.ValueString(),
		ExecStartPre:     toStringSlice(ctx, m.ExecStartPre, diags),
		ExecStartPost:    toStringSlice(ctx, m.ExecStartPost, diags),
		ExecStop:         m.ExecStop.ValueString(),
		ExecStopPost:     toStringSlice(ctx, m.ExecStopPost, diags),
		ExecReload:       m.ExecReload.ValueString(),
		Restart:          m.Restart.ValueString(),
		RestartSec:       m.RestartSec.ValueString(),
		TimeoutStartSec:  m.TimeoutStartSec.ValueString(),
		TimeoutStopSec:   m.TimeoutStopSec.ValueString(),
		User:             m.User.ValueString(),
		Group:            m.Group.ValueString(),
		WorkingDirectory: m.WorkingDirectory.ValueString(),
		Environment:      toStringMap(ctx, m.Environment, diags),
		EnvironmentFile:  m.EnvironmentFile.ValueString(),
		StandardOutput:   m.StandardOutput.ValueString(),
		StandardError:    m.StandardError.ValueString(),
		Extra:            toStringMap(ctx, m.Extra, diags),
	}

	if !m.RemainAfterExit.IsNull() && !m.RemainAfterExit.IsUnknown() {
		val := m.RemainAfterExit.ValueBool()
		s.RemainAfterExit = &val
	}

	return s
}

func (m *SystemdInstallSectionModel) toInstallSection(ctx context.Context, diags *diag.Diagnostics) *ssh.InstallSection {
	return &ssh.InstallSection{
		WantedBy:   toStringSlice(ctx, m.WantedBy, diags),
		RequiredBy: toStringSlice(ctx, m.RequiredBy, diags),
		Alias:      toStringSlice(ctx, m.Alias, diags),
		Extra:      toStringMap(ctx, m.Extra, diags),
	}
}

// --------------------------------------------------------------------
// SSH model → Terraform model
// --------------------------------------------------------------------

// applyServiceUnit writes an ssh.ServiceUnit back into the Terraform resource
// model, converting plain Go values to Terraform's typed attributes.
func (m *SystemdServiceResourceModel) applyServiceUnit(ctx context.Context, u *ssh.ServiceUnit, diags *diag.Diagnostics) {
	if u.Unit != nil {
		m.Unit = applyUnitSection(ctx, u.Unit, diags)
	} else {
		m.Unit = nil
	}
	if u.Service != nil {
		m.Service = applyServiceSection(ctx, u.Service, diags)
	} else {
		m.Service = nil
	}
	if u.Install != nil {
		m.Install = applyInstallSection(ctx, u.Install, diags)
	} else {
		m.Install = nil
	}
}

func applyUnitSection(ctx context.Context, s *ssh.UnitSection, diags *diag.Diagnostics) *SystemdUnitSectionModel {
	m := &SystemdUnitSectionModel{
		Description:   types.StringValue(s.Description),
		Documentation: fromStringSlice(ctx, s.Documentation, diags),
		After:         fromStringSlice(ctx, s.After, diags),
		Before:        fromStringSlice(ctx, s.Before, diags),
		Requires:      fromStringSlice(ctx, s.Requires, diags),
		Wants:         fromStringSlice(ctx, s.Wants, diags),
		BindsTo:       fromStringSlice(ctx, s.BindsTo, diags),
		PartOf:        fromStringSlice(ctx, s.PartOf, diags),
		Conflicts:     fromStringSlice(ctx, s.Conflicts, diags),
		Extra:         fromStringMap(ctx, s.Extra, diags),
	}

	if s.Condition != nil {
		m.Condition = applyCheckDirectives(ctx, s.Condition, diags)
	}
	if s.Assert != nil {
		m.Assert = applyCheckDirectives(ctx, s.Assert, diags)
	}

	return m
}

func applyCheckDirectives(ctx context.Context, c *ssh.CheckDirectives, diags *diag.Diagnostics) *SystemdCheckModel {
	return &SystemdCheckModel{
		PathExists:         fromStringSlice(ctx, c.PathExists, diags),
		PathIsDirectory:    fromStringSlice(ctx, c.PathIsDirectory, diags),
		PathIsSymbolicLink: fromStringSlice(ctx, c.PathIsSymbolicLink, diags),
		FileNotEmpty:       fromStringSlice(ctx, c.FileNotEmpty, diags),
		DirectoryNotEmpty:  fromStringSlice(ctx, c.DirectoryNotEmpty, diags),
		User:               fromGoString(c.User),
		Group:              fromGoString(c.Group),
		Host:               fromGoString(c.Host),
		Virtualization:     fromGoString(c.Virtualization),
		Security:           fromGoString(c.Security),
	}
}

func applyServiceSection(ctx context.Context, s *ssh.ServiceSection, diags *diag.Diagnostics) *SystemdServiceSectionModel {
	m := &SystemdServiceSectionModel{
		Type:             fromGoString(s.Type),
		ExecStart:        types.StringValue(s.ExecStart),
		ExecStartPre:     fromStringSlice(ctx, s.ExecStartPre, diags),
		ExecStartPost:    fromStringSlice(ctx, s.ExecStartPost, diags),
		ExecStop:         fromGoString(s.ExecStop),
		ExecStopPost:     fromStringSlice(ctx, s.ExecStopPost, diags),
		ExecReload:       fromGoString(s.ExecReload),
		Restart:          fromGoString(s.Restart),
		RestartSec:       fromGoString(s.RestartSec),
		TimeoutStartSec:  fromGoString(s.TimeoutStartSec),
		TimeoutStopSec:   fromGoString(s.TimeoutStopSec),
		User:             fromGoString(s.User),
		Group:            fromGoString(s.Group),
		WorkingDirectory: fromGoString(s.WorkingDirectory),
		Environment:      fromStringMap(ctx, s.Environment, diags),
		EnvironmentFile:  fromGoString(s.EnvironmentFile),
		StandardOutput:   fromGoString(s.StandardOutput),
		StandardError:    fromGoString(s.StandardError),
		Extra:            fromStringMap(ctx, s.Extra, diags),
	}

	if s.RemainAfterExit != nil {
		m.RemainAfterExit = types.BoolValue(*s.RemainAfterExit)
	}

	return m
}

func applyInstallSection(ctx context.Context, s *ssh.InstallSection, diags *diag.Diagnostics) *SystemdInstallSectionModel {
	return &SystemdInstallSectionModel{
		WantedBy:   fromStringSlice(ctx, s.WantedBy, diags),
		RequiredBy: fromStringSlice(ctx, s.RequiredBy, diags),
		Alias:      fromStringSlice(ctx, s.Alias, diags),
		Extra:      fromStringMap(ctx, s.Extra, diags),
	}
}

// --------------------------------------------------------------------
// Type conversion helpers
// --------------------------------------------------------------------

// toStringSlice converts a types.List to a Go string slice.
// Returns nil for null/unknown lists.
func toStringSlice(ctx context.Context, list types.List, diags *diag.Diagnostics) []string {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var values []string
	diags.Append(list.ElementsAs(ctx, &values, false)...)
	return values
}

// toStringMap converts a types.Map to a Go string map.
// Returns nil for null/unknown maps.
func toStringMap(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	var entries map[string]string
	diags.Append(m.ElementsAs(ctx, &entries, false)...)
	return entries
}

// fromStringSlice converts a Go string slice to a types.List.
// Returns a null list for nil slices.
func fromStringSlice(ctx context.Context, values []string, diags *diag.Diagnostics) types.List {
	if values == nil {
		return types.ListNull(types.StringType)
	}
	val, d := types.ListValueFrom(ctx, types.StringType, values)
	diags.Append(d...)
	return val
}

// fromStringMap converts a Go string map to a types.Map.
// Returns a null map for nil maps.
func fromStringMap(ctx context.Context, m map[string]string, diags *diag.Diagnostics) types.Map {
	if m == nil {
		return types.MapNull(types.StringType)
	}
	val, d := types.MapValueFrom(ctx, types.StringType, m)
	diags.Append(d...)
	return val
}

// fromGoString converts a Go string to a types.String.
// Returns null for empty strings.
func fromGoString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
