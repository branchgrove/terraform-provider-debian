package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/branchgrove/terraform-provider-debian/internal/ssh"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SystemdTimerResource{}
var _ resource.ResourceWithImportState = &SystemdTimerResource{}
var _ resource.ResourceWithValidateConfig = &SystemdTimerResource{}

func NewSystemdTimerResource() resource.Resource {
	return &SystemdTimerResource{}
}

type SystemdTimerResource struct {
	providerData *ProviderData
}

// --------------------------------------------------------------------
// Model types
// --------------------------------------------------------------------

type SystemdTimerResourceModel struct {
	Name       types.String                `tfsdk:"name"`
	Enabled    types.Bool                  `tfsdk:"enabled"`
	Active     types.Bool                  `tfsdk:"active"`
	Unit       *SystemdUnitSectionModel    `tfsdk:"unit"`
	Timer      *SystemdTimerSectionModel   `tfsdk:"timer"`
	Install    *SystemdInstallSectionModel `tfsdk:"install"`
	Connection ConnectionModel             `tfsdk:"ssh"`
}

type SystemdTimerSectionModel struct {
	OnCalendar         types.String `tfsdk:"on_calendar"`
	OnBootSec          types.String `tfsdk:"on_boot_sec"`
	OnStartupSec       types.String `tfsdk:"on_startup_sec"`
	OnUnitActiveSec    types.String `tfsdk:"on_unit_active_sec"`
	OnUnitInactiveSec  types.String `tfsdk:"on_unit_inactive_sec"`
	AccuracySec        types.String `tfsdk:"accuracy_sec"`
	RandomizedDelaySec types.String `tfsdk:"randomized_delay_sec"`
	Persistent         types.Bool   `tfsdk:"persistent"`
	WakeSystem         types.Bool   `tfsdk:"wake_system"`
	Unit               types.String `tfsdk:"unit"`
	Extra              types.Map    `tfsdk:"extra"`
}

// --------------------------------------------------------------------
// Resource interface
// --------------------------------------------------------------------

func (r *SystemdTimerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_systemd_timer"
}

func (r *SystemdTimerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
		MarkdownDescription: "`debian_systemd_timer` manages a systemd `.timer` unit on the remote host. A timer activates a corresponding `.service` unit on a schedule.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Timer unit name (without `.timer` extension).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the timer is enabled (`systemctl enable` / `disable`).",
				Required:            true,
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the timer is armed (`systemctl start` / `stop`).",
				Required:            true,
			},
			"unit": schema.SingleNestedAttribute{
				MarkdownDescription: "`[Unit]` section. Optional.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"description": schema.StringAttribute{
						MarkdownDescription: "`Description=` - human-readable description of the unit.",
						Required:            true,
					},
					"documentation": schema.ListAttribute{
						MarkdownDescription: "`Documentation=` - URIs referencing documentation.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"after": schema.ListAttribute{
						MarkdownDescription: "`After=` - units to order after.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"before": schema.ListAttribute{
						MarkdownDescription: "`Before=` - units to order before.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"requires": schema.ListAttribute{
						MarkdownDescription: "`Requires=` - hard dependencies.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"wants": schema.ListAttribute{
						MarkdownDescription: "`Wants=` - soft dependencies.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"binds_to": schema.ListAttribute{
						MarkdownDescription: "`BindsTo=` - stronger than Requires; stop if dependency stops.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"part_of": schema.ListAttribute{
						MarkdownDescription: "`PartOf=` - stop/restart when listed units stop/restart.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"conflicts": schema.ListAttribute{
						MarkdownDescription: "`Conflicts=` - units that cannot run simultaneously.",
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
			"timer": schema.SingleNestedAttribute{
				MarkdownDescription: "`[Timer]` section. Required - defines the schedule.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"on_calendar": schema.StringAttribute{
						MarkdownDescription: "`OnCalendar=` - realtime (wallclock) schedule expression, for example, `daily`, `hourly`, `*-*-* 03:00:00`.",
						Optional:            true,
					},
					"on_boot_sec": schema.StringAttribute{
						MarkdownDescription: "`OnBootSec=` - time after boot to first trigger.",
						Optional:            true,
					},
					"on_startup_sec": schema.StringAttribute{
						MarkdownDescription: "`OnStartupSec=` - time after systemd start to first trigger.",
						Optional:            true,
					},
					"on_unit_active_sec": schema.StringAttribute{
						MarkdownDescription: "`OnUnitActiveSec=` - time after the activated unit was last active.",
						Optional:            true,
					},
					"on_unit_inactive_sec": schema.StringAttribute{
						MarkdownDescription: "`OnUnitInactiveSec=` - time after the activated unit was last inactive.",
						Optional:            true,
					},
					"accuracy_sec": schema.StringAttribute{
						MarkdownDescription: "`AccuracySec=` - timer accuracy. Defaults to `1min`.",
						Optional:            true,
					},
					"randomized_delay_sec": schema.StringAttribute{
						MarkdownDescription: "`RandomizedDelaySec=` - random delay added to each trigger.",
						Optional:            true,
					},
					"persistent": schema.BoolAttribute{
						MarkdownDescription: "`Persistent=` - if true, catch up missed runs when the system was off.",
						Optional:            true,
					},
					"wake_system": schema.BoolAttribute{
						MarkdownDescription: "`WakeSystem=` - wake the system from suspend to fire the timer.",
						Optional:            true,
					},
					"unit": schema.StringAttribute{
						MarkdownDescription: "`Unit=` - service unit to activate. Defaults to `<name>.service` when omitted.",
						Optional:            true,
					},
					"extra": schema.MapAttribute{
						MarkdownDescription: "Additional `[Timer]` directives. Keys must use exact systemd directive names.",
						Optional:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"install": schema.SingleNestedAttribute{
				MarkdownDescription: "`[Install]` section. Optional.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"wanted_by": schema.ListAttribute{
						MarkdownDescription: "`WantedBy=` - targets that pull in this timer when enabled.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"required_by": schema.ListAttribute{
						MarkdownDescription: "`RequiredBy=` - targets that require this timer when enabled.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"alias": schema.ListAttribute{
						MarkdownDescription: "`Alias=` - alternative names for the unit.",
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
			"ssh": connectionSchema,
		},
	}
}

func (r *SystemdTimerResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data SystemdTimerResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Timer != nil {
		hasTrigger := false
		if !data.Timer.OnCalendar.IsNull() && !data.Timer.OnCalendar.IsUnknown() {
			hasTrigger = true
		}
		if !data.Timer.OnBootSec.IsNull() && !data.Timer.OnBootSec.IsUnknown() {
			hasTrigger = true
		}
		if !data.Timer.OnStartupSec.IsNull() && !data.Timer.OnStartupSec.IsUnknown() {
			hasTrigger = true
		}
		if !data.Timer.OnUnitActiveSec.IsNull() && !data.Timer.OnUnitActiveSec.IsUnknown() {
			hasTrigger = true
		}
		if !data.Timer.OnUnitInactiveSec.IsNull() && !data.Timer.OnUnitInactiveSec.IsUnknown() {
			hasTrigger = true
		}

		// Wait until all are known before throwing an error, otherwise we might complain prematurely.
		allKnown := !data.Timer.OnCalendar.IsUnknown() &&
			!data.Timer.OnBootSec.IsUnknown() &&
			!data.Timer.OnStartupSec.IsUnknown() &&
			!data.Timer.OnUnitActiveSec.IsUnknown() &&
			!data.Timer.OnUnitInactiveSec.IsUnknown()

		if allKnown && !hasTrigger {
			resp.Diagnostics.AddError(
				"Missing Timer Trigger",
				"At least one of on_calendar, on_boot_sec, on_startup_sec, on_unit_active_sec, or on_unit_inactive_sec must be set in the timer block.",
			)
		}
	}
}

func (r *SystemdTimerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SystemdTimerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SystemdTimerResourceModel

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

	unit := data.toTimerUnit(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := client.WriteTimerUnit(ctx, name, unit); err != nil {
		resp.Diagnostics.AddError("Failed to write unit file", err.Error())
		return
	}

	if err := client.DaemonReload(ctx); err != nil {
		resp.Diagnostics.AddError("Failed to daemon-reload", err.Error())
		return
	}

	if data.Enabled.ValueBool() {
		if err := client.EnableService(ctx, name+".timer"); err != nil {
			resp.Diagnostics.AddError("Failed to enable timer", err.Error())
			return
		}
	}

	if data.Active.ValueBool() {
		if err := client.StartService(ctx, name+".timer"); err != nil {
			resp.Diagnostics.AddError("Failed to start timer", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SystemdTimerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SystemdTimerResourceModel

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

	unit, err := client.ReadTimerUnit(ctx, name)
	if err != nil {
		if errors.Is(err, ssh.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read unit file", err.Error())
		return
	}

	data.applyTimerUnit(ctx, unit, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	state, err := client.GetServiceState(ctx, name+".timer")
	if err != nil {
		resp.Diagnostics.AddError("Failed to get timer state", err.Error())
		return
	}

	data.Enabled = types.BoolValue(state.Enabled)
	data.Active = types.BoolValue(state.Active)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SystemdTimerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SystemdTimerResourceModel

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

	planUnit := plan.toTimerUnit(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	planContent := planUnit.Serialize()
	stateContent := ""

	stateUnit := state.toTimerUnit(ctx, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		stateContent = stateUnit.Serialize()
	}

	unitFileChanged = planContent != stateContent

	if unitFileChanged {
		if err := client.WriteTimerUnit(ctx, name, planUnit); err != nil {
			resp.Diagnostics.AddError("Failed to write unit file", err.Error())
			return
		}

		if err := client.DaemonReload(ctx); err != nil {
			resp.Diagnostics.AddError("Failed to daemon-reload", err.Error())
			return
		}
	}

	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		if plan.Enabled.ValueBool() {
			if err := client.EnableService(ctx, name+".timer"); err != nil {
				resp.Diagnostics.AddError("Failed to enable timer", err.Error())
				return
			}
		} else {
			if err := client.DisableService(ctx, name+".timer"); err != nil {
				resp.Diagnostics.AddError("Failed to disable timer", err.Error())
				return
			}
		}
	}

	if plan.Active.ValueBool() != state.Active.ValueBool() {
		if plan.Active.ValueBool() {
			if err := client.StartService(ctx, name+".timer"); err != nil {
				resp.Diagnostics.AddError("Failed to start timer", err.Error())
				return
			}
		} else {
			if err := client.StopService(ctx, name+".timer"); err != nil {
				resp.Diagnostics.AddError("Failed to stop timer", err.Error())
				return
			}
		}
	} else if unitFileChanged && plan.Active.ValueBool() {
		if err := client.RestartService(ctx, name+".timer"); err != nil {
			resp.Diagnostics.AddError("Failed to restart timer", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SystemdTimerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SystemdTimerResourceModel

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
		if err := client.StopService(ctx, name+".timer"); err != nil {
			resp.Diagnostics.AddError("Failed to stop timer", err.Error())
			return
		}
	}

	if data.Enabled.ValueBool() {
		if err := client.DisableService(ctx, name+".timer"); err != nil {
			resp.Diagnostics.AddError("Failed to disable timer", err.Error())
			return
		}
	}

	if err := client.DeleteTimerUnit(ctx, name); err != nil {
		resp.Diagnostics.AddError("Failed to delete unit file", err.Error())
		return
	}
	if err := client.DaemonReload(ctx); err != nil {
		resp.Diagnostics.AddError("Failed to daemon-reload", err.Error())
		return
	}
}

func (r *SystemdTimerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	user, publicKey, host, port, timerName, err := parseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	if timerName == "" {
		resp.Diagnostics.AddError("Invalid import ID", "timer name must not be empty")
		return
	}

	timerName = strings.TrimSuffix(timerName, ".timer")

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), timerName)...)

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

func (m *SystemdTimerResourceModel) toTimerUnit(ctx context.Context, diags *diag.Diagnostics) *ssh.TimerUnit {
	u := &ssh.TimerUnit{}

	if m.Unit != nil {
		u.Unit = m.Unit.toUnitSection(ctx, diags)
	}
	if m.Timer != nil {
		u.Timer = m.Timer.toTimerSection(ctx, diags)
	}
	if m.Install != nil {
		u.Install = m.Install.toInstallSection(ctx, diags)
	}

	return u
}

func (m *SystemdTimerSectionModel) toTimerSection(ctx context.Context, diags *diag.Diagnostics) *ssh.TimerSection {
	s := &ssh.TimerSection{
		OnCalendar:         m.OnCalendar.ValueString(),
		OnBootSec:          m.OnBootSec.ValueString(),
		OnStartupSec:       m.OnStartupSec.ValueString(),
		OnUnitActiveSec:    m.OnUnitActiveSec.ValueString(),
		OnUnitInactiveSec:  m.OnUnitInactiveSec.ValueString(),
		AccuracySec:        m.AccuracySec.ValueString(),
		RandomizedDelaySec: m.RandomizedDelaySec.ValueString(),
		Unit:               m.Unit.ValueString(),
		Extra:              toStringMap(ctx, m.Extra, diags),
	}

	if !m.Persistent.IsNull() && !m.Persistent.IsUnknown() {
		val := m.Persistent.ValueBool()
		s.Persistent = &val
	}

	if !m.WakeSystem.IsNull() && !m.WakeSystem.IsUnknown() {
		val := m.WakeSystem.ValueBool()
		s.WakeSystem = &val
	}

	return s
}

// --------------------------------------------------------------------
// SSH model → Terraform model
// --------------------------------------------------------------------

func (m *SystemdTimerResourceModel) applyTimerUnit(ctx context.Context, u *ssh.TimerUnit, diags *diag.Diagnostics) {
	if u.Unit != nil {
		m.Unit = applyUnitSection(ctx, u.Unit, diags)
	} else {
		m.Unit = nil
	}
	if u.Timer != nil {
		m.Timer = applyTimerSection(ctx, u.Timer, diags)
	} else {
		m.Timer = nil
	}
	if u.Install != nil {
		m.Install = applyInstallSection(ctx, u.Install, diags)
	} else {
		m.Install = nil
	}
}

func applyTimerSection(ctx context.Context, s *ssh.TimerSection, diags *diag.Diagnostics) *SystemdTimerSectionModel {
	m := &SystemdTimerSectionModel{
		OnCalendar:         fromGoString(s.OnCalendar),
		OnBootSec:          fromGoString(s.OnBootSec),
		OnStartupSec:       fromGoString(s.OnStartupSec),
		OnUnitActiveSec:    fromGoString(s.OnUnitActiveSec),
		OnUnitInactiveSec:  fromGoString(s.OnUnitInactiveSec),
		AccuracySec:        fromGoString(s.AccuracySec),
		RandomizedDelaySec: fromGoString(s.RandomizedDelaySec),
		Unit:               fromGoString(s.Unit),
		Extra:              fromStringMap(ctx, s.Extra, diags),
	}

	if s.Persistent != nil {
		m.Persistent = types.BoolValue(*s.Persistent)
	}

	if s.WakeSystem != nil {
		m.WakeSystem = types.BoolValue(*s.WakeSystem)
	}

	return m
}
