package ssh

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ServiceState holds the enabled and active state of a systemd service.
type ServiceState struct {
	Enabled bool
	Active  bool
}

// ServiceUnit represents a systemd .service unit file as structured data.
// It can be serialized to and deserialized from the standard INI format.
type ServiceUnit struct {
	Unit    *UnitSection
	Service *ServiceSection
	Install *InstallSection
}

// UnitSection represents the [Unit] section of a systemd unit file.
type UnitSection struct {
	Description   string
	Documentation []string
	After         []string
	Before        []string
	Requires      []string
	Wants         []string
	BindsTo       []string
	PartOf        []string
	Conflicts     []string
	Condition     *CheckDirectives
	Assert        *CheckDirectives
	Extra         map[string]string
}

// CheckDirectives represents Condition* or Assert* directives in the [Unit] section.
type CheckDirectives struct {
	PathExists         []string
	PathIsDirectory    []string
	PathIsSymbolicLink []string
	FileNotEmpty       []string
	DirectoryNotEmpty  []string
	User               string
	Group              string
	Host               string
	Virtualization     string
	Security           string
}

// ServiceSection represents the [Service] section of a systemd unit file.
type ServiceSection struct {
	Type             string
	ExecStart        string
	ExecStartPre     []string
	ExecStartPost    []string
	ExecStop         string
	ExecStopPost     []string
	ExecReload       string
	Restart          string
	RestartSec       string
	TimeoutStartSec  string
	TimeoutStopSec   string
	User             string
	Group            string
	WorkingDirectory string
	Environment      map[string]string
	EnvironmentFile  string
	StandardOutput   string
	StandardError    string
	RemainAfterExit  *bool
	Extra            map[string]string
}

// InstallSection represents the [Install] section of a systemd unit file.
type InstallSection struct {
	WantedBy   []string
	RequiredBy []string
	Alias      []string
	Extra      map[string]string
}

// TimerUnit represents a systemd .timer unit file as structured data.
// It can be serialized to and deserialized from the standard INI format.
type TimerUnit struct {
	Unit    *UnitSection
	Timer   *TimerSection
	Install *InstallSection
}

// TimerSection represents the [Timer] section of a systemd timer unit file.
type TimerSection struct {
	OnCalendar         string
	OnBootSec          string
	OnStartupSec       string
	OnUnitActiveSec    string
	OnUnitInactiveSec  string
	AccuracySec        string
	RandomizedDelaySec string
	Persistent         *bool
	WakeSystem         *bool
	Unit               string
	Extra              map[string]string
}

// unitFilePath returns the path to the unit file for the named service under
// /etc/systemd/system.
func unitFilePath(name string) string {
	return "/etc/systemd/system/" + name + ".service"
}

// timerUnitFilePath returns the path to the timer unit file for the named timer under
// /etc/systemd/system.
func timerUnitFilePath(name string) string {
	return "/etc/systemd/system/" + name + ".timer"
}

// GetServiceState queries systemctl to determine whether the named service
// is enabled and active.
func (c *Client) GetServiceState(ctx context.Context, name string) (*ServiceState, error) {
	env := map[string]string{"NAME": name}

	res, err := c.Run(ctx, `systemctl is-enabled "$NAME" 2>/dev/null || true`, env, nil)
	if err != nil {
		return nil, fmt.Errorf("get service state: is-enabled %q: %w", name, err)
	}
	enabled := strings.TrimSpace(string(res.Stdout)) == "enabled"

	res, err = c.Run(ctx, `systemctl is-active "$NAME" 2>/dev/null || true`, env, nil)
	if err != nil {
		return nil, fmt.Errorf("get service state: is-active %q: %w", name, err)
	}
	active := strings.TrimSpace(string(res.Stdout)) == "active"

	return &ServiceState{Enabled: enabled, Active: active}, nil
}

// EnableService runs systemctl enable for the named service.
func (c *Client) EnableService(ctx context.Context, name string) error {
	env := map[string]string{"NAME": name}
	_, err := c.Run(ctx, `systemctl enable "$NAME"`, env, nil)
	if err != nil {
		return fmt.Errorf("enable service %q: %w", name, err)
	}
	return nil
}

// DisableService runs systemctl disable for the named service.
func (c *Client) DisableService(ctx context.Context, name string) error {
	env := map[string]string{"NAME": name}
	_, err := c.Run(ctx, `systemctl disable "$NAME"`, env, nil)
	if err != nil {
		return fmt.Errorf("disable service %q: %w", name, err)
	}
	return nil
}

// StartService runs systemctl start for the named service.
func (c *Client) StartService(ctx context.Context, name string) error {
	env := map[string]string{"NAME": name}
	_, err := c.Run(ctx, `systemctl start "$NAME"`, env, nil)
	if err != nil {
		return fmt.Errorf("start service %q: %w", name, err)
	}
	return nil
}

// WaitServiceActive waits for the named service to become active by polling its state.
// It returns an error if the service fails to start or times out.
func (c *Client) WaitServiceActive(ctx context.Context, name string, timeout time.Duration) error {
	// Short delay to catch immediate crashes of Type=simple services
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(1 * time.Second):
	}

	deadline := time.Now().Add(timeout)
	env := map[string]string{"NAME": name}

	for time.Now().Before(deadline) {
		res, err := c.Run(ctx, `systemctl show -p ActiveState "$NAME" | cut -d= -f2`, env, nil)
		if err != nil {
			return fmt.Errorf("wait service active %q: check state failed: %w", name, err)
		}

		state := strings.TrimSpace(string(res.Stdout))
		switch state {
		case "failed":
			statusRes, err := c.Run(ctx, `systemctl status "$NAME"`, env, nil)
			var stdout, stderr string
			if err != nil {
				if runErr, ok := err.(*RunError); ok {
					stdout = string(runErr.Stdout)
					stderr = string(runErr.Stderr)
				} else {
					return fmt.Errorf("service %q failed to start, and failed to get status: %w", name, err)
				}
			} else {
				stdout = string(statusRes.Stdout)
				stderr = string(statusRes.Stderr)
			}
			logs := strings.TrimSpace(stderr + "\n" + stdout)
			return fmt.Errorf("service %q failed to start:\n%s", name, logs)
		case "active", "inactive":
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	return fmt.Errorf("timeout waiting for service %q to become active", name)
}

// StopService runs systemctl stop for the named service.
func (c *Client) StopService(ctx context.Context, name string) error {
	env := map[string]string{"NAME": name}
	_, err := c.Run(ctx, `systemctl stop "$NAME"`, env, nil)
	if err != nil {
		return fmt.Errorf("stop service %q: %w", name, err)
	}
	return nil
}

// RestartService runs systemctl restart for the named service.
func (c *Client) RestartService(ctx context.Context, name string) error {
	env := map[string]string{"NAME": name}
	_, err := c.Run(ctx, `systemctl restart "$NAME"`, env, nil)
	if err != nil {
		return fmt.Errorf("restart service %q: %w", name, err)
	}
	return nil
}

// DaemonReload runs systemctl daemon-reload to re-read all unit files.
func (c *Client) DaemonReload(ctx context.Context) error {
	_, err := c.Run(ctx, `systemctl daemon-reload`, nil, nil)
	if err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}
	return nil
}

// WriteServiceUnit writes a ServiceUnit to /etc/systemd/system/<name>.service
// using PutFile for atomic writes.
func (c *Client) WriteServiceUnit(ctx context.Context, name string, unit *ServiceUnit) error {
	content := unit.Serialize()
	_, err := c.PutFile(ctx, &PutFileCommand{
		Path:    unitFilePath(name),
		Content: strings.NewReader(content),
		Mode:    "0644",
	})
	if err != nil {
		return fmt.Errorf("write service unit %q: %w", name, err)
	}
	return nil
}

// ReadServiceUnit reads and parses a ServiceUnit from
// /etc/systemd/system/<name>.service. Returns ErrNotFound if the file does
// not exist.
func (c *Client) ReadServiceUnit(ctx context.Context, name string) (*ServiceUnit, error) {
	path := unitFilePath(name)
	content, err := c.ReadFile(ctx, path, 64*1024)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("read service unit %q: %w", name, ErrNotFound)
		}
		return nil, fmt.Errorf("read service unit %q: %w", name, err)
	}
	unit := ParseServiceUnit(content)
	return unit, nil
}

// ServiceUnitExists checks whether a unit file exists at
// /etc/systemd/system/<name>.service.
func (c *Client) ServiceUnitExists(ctx context.Context, name string) (bool, error) {
	env := map[string]string{"FILE": unitFilePath(name)}
	_, err := c.Run(ctx, `test -f "$FILE"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return false, nil
		}
		return false, fmt.Errorf("service unit exists %q: %w", name, err)
	}
	return true, nil
}

// DeleteServiceUnit removes the unit file at
// /etc/systemd/system/<name>.service.
func (c *Client) DeleteServiceUnit(ctx context.Context, name string) error {
	err := c.DeleteFile(ctx, unitFilePath(name))
	if err != nil {
		return fmt.Errorf("delete service unit %q: %w", name, err)
	}
	return nil
}

// WriteTimerUnit writes a TimerUnit to /etc/systemd/system/<name>.timer
// using PutFile for atomic writes.
func (c *Client) WriteTimerUnit(ctx context.Context, name string, unit *TimerUnit) error {
	content := unit.Serialize()
	_, err := c.PutFile(ctx, &PutFileCommand{
		Path:    timerUnitFilePath(name),
		Content: strings.NewReader(content),
		Mode:    "0644",
	})
	if err != nil {
		return fmt.Errorf("write timer unit %q: %w", name, err)
	}
	return nil
}

// ReadTimerUnit reads and parses a TimerUnit from
// /etc/systemd/system/<name>.timer. Returns ErrNotFound if the file does
// not exist.
func (c *Client) ReadTimerUnit(ctx context.Context, name string) (*TimerUnit, error) {
	path := timerUnitFilePath(name)
	content, err := c.ReadFile(ctx, path, 64*1024)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("read timer unit %q: %w", name, ErrNotFound)
		}
		return nil, fmt.Errorf("read timer unit %q: %w", name, err)
	}
	unit := ParseTimerUnit(content)
	return unit, nil
}

// TimerUnitExists checks whether a unit file exists at
// /etc/systemd/system/<name>.timer.
func (c *Client) TimerUnitExists(ctx context.Context, name string) (bool, error) {
	env := map[string]string{"FILE": timerUnitFilePath(name)}
	_, err := c.Run(ctx, `test -f "$FILE"`, env, nil)
	if err != nil {
		if _, ok := errors.AsType[*RunError](err); ok {
			return false, nil
		}
		return false, fmt.Errorf("timer unit exists %q: %w", name, err)
	}
	return true, nil
}

// DeleteTimerUnit removes the unit file at
// /etc/systemd/system/<name>.timer.
func (c *Client) DeleteTimerUnit(ctx context.Context, name string) error {
	err := c.DeleteFile(ctx, timerUnitFilePath(name))
	if err != nil {
		return fmt.Errorf("delete timer unit %q: %w", name, err)
	}
	return nil
}

// --------------------------------------------------------------------
// Unit file serialization
// --------------------------------------------------------------------

// unitFileBuilder writes systemd unit file INI format.
type unitFileBuilder struct {
	buf     strings.Builder
	started bool
}

func (b *unitFileBuilder) section(name string) {
	if b.started {
		b.buf.WriteByte('\n')
	}
	b.buf.WriteByte('[')
	b.buf.WriteString(name)
	b.buf.WriteString("]\n")
	b.started = true
}

func (b *unitFileBuilder) directive(name, value string) {
	b.buf.WriteString(name)
	b.buf.WriteByte('=')
	b.buf.WriteString(value)
	b.buf.WriteByte('\n')
}

func (b *unitFileBuilder) writeString(name, value string) {
	if value != "" {
		b.directive(name, value)
	}
}

func (b *unitFileBuilder) writeList(name string, values []string) {
	for _, v := range values {
		b.directive(name, v)
	}
}

func (b *unitFileBuilder) writeMapSorted(m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		b.directive(k, m[k])
	}
}

// Serialize converts the ServiceUnit to the systemd unit file INI format.
func (u *ServiceUnit) Serialize() string {
	var b unitFileBuilder

	if u.Unit != nil {
		b.section("Unit")
		u.Unit.serialize(&b)
	}
	if u.Service != nil {
		b.section("Service")
		u.Service.serialize(&b)
	}
	if u.Install != nil {
		b.section("Install")
		u.Install.serialize(&b)
	}

	return b.buf.String()
}

// Serialize converts the TimerUnit to the systemd unit file INI format.
func (u *TimerUnit) Serialize() string {
	var b unitFileBuilder

	if u.Unit != nil {
		b.section("Unit")
		u.Unit.serialize(&b)
	}
	if u.Timer != nil {
		b.section("Timer")
		u.Timer.serialize(&b)
	}
	if u.Install != nil {
		b.section("Install")
		u.Install.serialize(&b)
	}

	return b.buf.String()
}

func (s *UnitSection) serialize(b *unitFileBuilder) {
	b.writeString("Description", s.Description)
	b.writeList("Documentation", s.Documentation)
	b.writeList("After", s.After)
	b.writeList("Before", s.Before)
	b.writeList("Requires", s.Requires)
	b.writeList("Wants", s.Wants)
	b.writeList("BindsTo", s.BindsTo)
	b.writeList("PartOf", s.PartOf)
	b.writeList("Conflicts", s.Conflicts)

	if s.Condition != nil {
		s.Condition.serialize(b, "Condition")
	}
	if s.Assert != nil {
		s.Assert.serialize(b, "Assert")
	}

	b.writeMapSorted(s.Extra)
}

func (c *CheckDirectives) serialize(b *unitFileBuilder, prefix string) {
	b.writeList(prefix+"PathExists", c.PathExists)
	b.writeList(prefix+"PathIsDirectory", c.PathIsDirectory)
	b.writeList(prefix+"PathIsSymbolicLink", c.PathIsSymbolicLink)
	b.writeList(prefix+"FileNotEmpty", c.FileNotEmpty)
	b.writeList(prefix+"DirectoryNotEmpty", c.DirectoryNotEmpty)
	b.writeString(prefix+"User", c.User)
	b.writeString(prefix+"Group", c.Group)
	b.writeString(prefix+"Host", c.Host)
	b.writeString(prefix+"Virtualization", c.Virtualization)
	b.writeString(prefix+"Security", c.Security)
}

func (s *ServiceSection) serialize(b *unitFileBuilder) {
	b.writeString("Type", s.Type)
	b.writeString("ExecStart", s.ExecStart)
	b.writeList("ExecStartPre", s.ExecStartPre)
	b.writeList("ExecStartPost", s.ExecStartPost)
	b.writeString("ExecStop", s.ExecStop)
	b.writeList("ExecStopPost", s.ExecStopPost)
	b.writeString("ExecReload", s.ExecReload)
	b.writeString("Restart", s.Restart)
	b.writeString("RestartSec", s.RestartSec)
	b.writeString("TimeoutStartSec", s.TimeoutStartSec)
	b.writeString("TimeoutStopSec", s.TimeoutStopSec)
	b.writeString("User", s.User)
	b.writeString("Group", s.Group)
	b.writeString("WorkingDirectory", s.WorkingDirectory)

	if len(s.Environment) > 0 {
		keys := make([]string, 0, len(s.Environment))
		for k := range s.Environment {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			b.directive("Environment", fmt.Sprintf(`"%s=%s"`, k, s.Environment[k]))
		}
	}

	b.writeString("EnvironmentFile", s.EnvironmentFile)
	b.writeString("StandardOutput", s.StandardOutput)
	b.writeString("StandardError", s.StandardError)

	if s.RemainAfterExit != nil {
		if *s.RemainAfterExit {
			b.directive("RemainAfterExit", "yes")
		} else {
			b.directive("RemainAfterExit", "no")
		}
	}

	b.writeMapSorted(s.Extra)
}

func (s *TimerSection) serialize(b *unitFileBuilder) {
	b.writeString("OnCalendar", s.OnCalendar)
	b.writeString("OnBootSec", s.OnBootSec)
	b.writeString("OnStartupSec", s.OnStartupSec)
	b.writeString("OnUnitActiveSec", s.OnUnitActiveSec)
	b.writeString("OnUnitInactiveSec", s.OnUnitInactiveSec)
	b.writeString("AccuracySec", s.AccuracySec)
	b.writeString("RandomizedDelaySec", s.RandomizedDelaySec)

	if s.Persistent != nil {
		if *s.Persistent {
			b.directive("Persistent", "yes")
		} else {
			b.directive("Persistent", "no")
		}
	}

	if s.WakeSystem != nil {
		if *s.WakeSystem {
			b.directive("WakeSystem", "yes")
		} else {
			b.directive("WakeSystem", "no")
		}
	}

	b.writeString("Unit", s.Unit)

	b.writeMapSorted(s.Extra)
}

func (s *InstallSection) serialize(b *unitFileBuilder) {
	b.writeList("WantedBy", s.WantedBy)
	b.writeList("RequiredBy", s.RequiredBy)
	b.writeList("Alias", s.Alias)
	b.writeMapSorted(s.Extra)
}

// --------------------------------------------------------------------
// Unit file deserialization
// --------------------------------------------------------------------

// unitDirective is a parsed key=value pair from a unit file.
type unitDirective struct {
	Name  string
	Value string
}

// parseUnitFile parses systemd INI format into sections of directives.
func parseUnitFile(content string) map[string][]unitDirective {
	sections := map[string][]unitDirective{}
	currentSection := ""
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			continue
		}
		if currentSection == "" {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		sections[currentSection] = append(sections[currentSection], unitDirective{
			Name:  strings.TrimSpace(name),
			Value: strings.TrimSpace(value),
		})
	}
	return sections
}

// ParseServiceUnit parses a systemd unit file string into a ServiceUnit.
func ParseServiceUnit(content string) *ServiceUnit {
	sections := parseUnitFile(content)
	u := &ServiceUnit{}

	if directives, ok := sections["Unit"]; ok && len(directives) > 0 {
		u.Unit = parseUnitSection(directives)
	}
	if directives, ok := sections["Service"]; ok && len(directives) > 0 {
		u.Service = parseServiceSection(directives)
	}
	if directives, ok := sections["Install"]; ok && len(directives) > 0 {
		u.Install = parseInstallSection(directives)
	}

	return u
}

// ParseTimerUnit parses a systemd timer unit file string into a TimerUnit.
func ParseTimerUnit(content string) *TimerUnit {
	contentStr := parseUnitFile(content)
	sections := contentStr
	u := &TimerUnit{}

	if directives, ok := sections["Unit"]; ok && len(directives) > 0 {
		u.Unit = parseUnitSection(directives)
	}
	if directives, ok := sections["Timer"]; ok && len(directives) > 0 {
		u.Timer = parseTimerSection(directives)
	}
	if directives, ok := sections["Install"]; ok && len(directives) > 0 {
		u.Install = parseInstallSection(directives)
	}

	return u
}

func parseUnitSection(directives []unitDirective) *UnitSection {
	s := &UnitSection{}
	extra := map[string]string{}

	condLists := map[string][]string{}
	condStrings := map[string]string{}
	assertLists := map[string][]string{}
	assertStrings := map[string]string{}

	for _, d := range directives {
		switch d.Name {
		case "Description":
			s.Description = d.Value
		case "Documentation":
			s.Documentation = append(s.Documentation, d.Value)
		case "After":
			s.After = append(s.After, d.Value)
		case "Before":
			s.Before = append(s.Before, d.Value)
		case "Requires":
			s.Requires = append(s.Requires, d.Value)
		case "Wants":
			s.Wants = append(s.Wants, d.Value)
		case "BindsTo":
			s.BindsTo = append(s.BindsTo, d.Value)
		case "PartOf":
			s.PartOf = append(s.PartOf, d.Value)
		case "Conflicts":
			s.Conflicts = append(s.Conflicts, d.Value)
		case "ConditionPathExists", "ConditionPathIsDirectory", "ConditionPathIsSymbolicLink",
			"ConditionFileNotEmpty", "ConditionDirectoryNotEmpty":
			condLists[d.Name] = append(condLists[d.Name], d.Value)
		case "ConditionUser":
			condStrings["User"] = d.Value
		case "ConditionGroup":
			condStrings["Group"] = d.Value
		case "ConditionHost":
			condStrings["Host"] = d.Value
		case "ConditionVirtualization":
			condStrings["Virtualization"] = d.Value
		case "ConditionSecurity":
			condStrings["Security"] = d.Value
		case "AssertPathExists", "AssertPathIsDirectory", "AssertPathIsSymbolicLink",
			"AssertFileNotEmpty", "AssertDirectoryNotEmpty":
			assertLists[d.Name] = append(assertLists[d.Name], d.Value)
		case "AssertUser":
			assertStrings["User"] = d.Value
		case "AssertGroup":
			assertStrings["Group"] = d.Value
		case "AssertHost":
			assertStrings["Host"] = d.Value
		case "AssertVirtualization":
			assertStrings["Virtualization"] = d.Value
		case "AssertSecurity":
			assertStrings["Security"] = d.Value
		default:
			extra[d.Name] = d.Value
		}
	}

	if len(condLists) > 0 || len(condStrings) > 0 {
		s.Condition = parseCheckDirectives("Condition", condLists, condStrings)
	}
	if len(assertLists) > 0 || len(assertStrings) > 0 {
		s.Assert = parseCheckDirectives("Assert", assertLists, assertStrings)
	}

	if len(extra) > 0 {
		s.Extra = extra
	}

	return s
}

func parseCheckDirectives(prefix string, lists map[string][]string, strs map[string]string) *CheckDirectives {
	c := &CheckDirectives{
		PathExists:         lists[prefix+"PathExists"],
		PathIsDirectory:    lists[prefix+"PathIsDirectory"],
		PathIsSymbolicLink: lists[prefix+"PathIsSymbolicLink"],
		FileNotEmpty:       lists[prefix+"FileNotEmpty"],
		DirectoryNotEmpty:  lists[prefix+"DirectoryNotEmpty"],
		User:               strs["User"],
		Group:              strs["Group"],
		Host:               strs["Host"],
		Virtualization:     strs["Virtualization"],
		Security:           strs["Security"],
	}
	return c
}

func parseServiceSection(directives []unitDirective) *ServiceSection {
	s := &ServiceSection{}
	extra := map[string]string{}

	for _, d := range directives {
		switch d.Name {
		case "Type":
			s.Type = d.Value
		case "ExecStart":
			s.ExecStart = d.Value
		case "ExecStartPre":
			s.ExecStartPre = append(s.ExecStartPre, d.Value)
		case "ExecStartPost":
			s.ExecStartPost = append(s.ExecStartPost, d.Value)
		case "ExecStop":
			s.ExecStop = d.Value
		case "ExecStopPost":
			s.ExecStopPost = append(s.ExecStopPost, d.Value)
		case "ExecReload":
			s.ExecReload = d.Value
		case "Restart":
			s.Restart = d.Value
		case "RestartSec":
			s.RestartSec = d.Value
		case "TimeoutStartSec":
			s.TimeoutStartSec = d.Value
		case "TimeoutStopSec":
			s.TimeoutStopSec = d.Value
		case "User":
			s.User = d.Value
		case "Group":
			s.Group = d.Value
		case "WorkingDirectory":
			s.WorkingDirectory = d.Value
		case "Environment":
			k, v := parseEnvironmentDirective(d.Value)
			if k != "" {
				if s.Environment == nil {
					s.Environment = map[string]string{}
				}
				s.Environment[k] = v
			}
		case "EnvironmentFile":
			s.EnvironmentFile = d.Value
		case "StandardOutput":
			s.StandardOutput = d.Value
		case "StandardError":
			s.StandardError = d.Value
		case "RemainAfterExit":
			val := parseBoolDirective(d.Value)
			s.RemainAfterExit = &val
		default:
			extra[d.Name] = d.Value
		}
	}

	if len(extra) > 0 {
		s.Extra = extra
	}

	return s
}

func parseTimerSection(directives []unitDirective) *TimerSection {
	s := &TimerSection{}
	extra := map[string]string{}

	for _, d := range directives {
		switch d.Name {
		case "OnCalendar":
			s.OnCalendar = d.Value
		case "OnBootSec":
			s.OnBootSec = d.Value
		case "OnStartupSec":
			s.OnStartupSec = d.Value
		case "OnUnitActiveSec":
			s.OnUnitActiveSec = d.Value
		case "OnUnitInactiveSec":
			s.OnUnitInactiveSec = d.Value
		case "AccuracySec":
			s.AccuracySec = d.Value
		case "RandomizedDelaySec":
			s.RandomizedDelaySec = d.Value
		case "Persistent":
			val := parseBoolDirective(d.Value)
			s.Persistent = &val
		case "WakeSystem":
			val := parseBoolDirective(d.Value)
			s.WakeSystem = &val
		case "Unit":
			s.Unit = d.Value
		default:
			extra[d.Name] = d.Value
		}
	}

	if len(extra) > 0 {
		s.Extra = extra
	}

	return s
}

func parseInstallSection(directives []unitDirective) *InstallSection {
	s := &InstallSection{}
	extra := map[string]string{}

	for _, d := range directives {
		switch d.Name {
		case "WantedBy":
			s.WantedBy = append(s.WantedBy, d.Value)
		case "RequiredBy":
			s.RequiredBy = append(s.RequiredBy, d.Value)
		case "Alias":
			s.Alias = append(s.Alias, d.Value)
		default:
			extra[d.Name] = d.Value
		}
	}

	if len(extra) > 0 {
		s.Extra = extra
	}

	return s
}

// parseEnvironmentDirective extracts key and value from an Environment=
// directive value such as `"FOO=bar"` or `FOO=bar`.
func parseEnvironmentDirective(raw string) (string, string) {
	v := strings.TrimSpace(raw)
	if strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
		v = v[1 : len(v)-1]
	}
	parts := strings.SplitN(v, "=", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// parseBoolDirective converts systemd boolean strings to a Go bool.
func parseBoolDirective(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "yes", "true", "1", "on":
		return true
	default:
		return false
	}
}
