package ssh

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------
// Serialization unit tests (no SSH required)
// --------------------------------------------------------------------

func TestSerializeServiceUnit(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		u := &ServiceUnit{
			Unit: &UnitSection{
				Description: "Test service",
			},
			Service: &ServiceSection{
				ExecStart: "/bin/true",
			},
		}

		got := u.Serialize()
		assert.Equal(t, "[Unit]\nDescription=Test service\n\n[Service]\nExecStart=/bin/true\n", got)
	})

	t.Run("all sections", func(t *testing.T) {
		u := &ServiceUnit{
			Unit: &UnitSection{
				Description: "Full service",
			},
			Service: &ServiceSection{
				ExecStart: "/usr/bin/myapp",
			},
			Install: &InstallSection{
				WantedBy: []string{"multi-user.target"},
			},
		}

		got := u.Serialize()
		assert.Contains(t, got, "[Unit]\n")
		assert.Contains(t, got, "[Service]\n")
		assert.Contains(t, got, "[Install]\n")
		assert.Contains(t, got, "WantedBy=multi-user.target\n")
	})

	t.Run("unit section list directives", func(t *testing.T) {
		u := &ServiceUnit{
			Unit: &UnitSection{
				Description:   "Lists test",
				Documentation: []string{"https://example.com", "man:myapp(1)"},
				After:         []string{"network.target", "syslog.target"},
				Before:        []string{"shutdown.target"},
				Requires:      []string{"network.target"},
				Wants:         []string{"network-online.target"},
				BindsTo:       []string{"other.service"},
				PartOf:        []string{"parent.service"},
				Conflicts:     []string{"conflicting.service"},
			},
		}

		got := u.Serialize()
		assert.Contains(t, got, "Documentation=https://example.com\n")
		assert.Contains(t, got, "Documentation=man:myapp(1)\n")
		assert.Contains(t, got, "After=network.target\n")
		assert.Contains(t, got, "After=syslog.target\n")
		assert.Contains(t, got, "Before=shutdown.target\n")
		assert.Contains(t, got, "Requires=network.target\n")
		assert.Contains(t, got, "Wants=network-online.target\n")
		assert.Contains(t, got, "BindsTo=other.service\n")
		assert.Contains(t, got, "PartOf=parent.service\n")
		assert.Contains(t, got, "Conflicts=conflicting.service\n")
	})

	t.Run("condition and assert directives", func(t *testing.T) {
		u := &ServiceUnit{
			Unit: &UnitSection{
				Description: "Checks test",
				Condition: &CheckDirectives{
					PathExists:      []string{"/etc/myapp.conf"},
					PathIsDirectory: []string{"/var/lib/myapp"},
					User:            "root",
				},
				Assert: &CheckDirectives{
					FileNotEmpty: []string{"/etc/myapp.conf"},
					Host:         "myhost",
				},
			},
		}

		got := u.Serialize()
		assert.Contains(t, got, "ConditionPathExists=/etc/myapp.conf\n")
		assert.Contains(t, got, "ConditionPathIsDirectory=/var/lib/myapp\n")
		assert.Contains(t, got, "ConditionUser=root\n")
		assert.Contains(t, got, "AssertFileNotEmpty=/etc/myapp.conf\n")
		assert.Contains(t, got, "AssertHost=myhost\n")
	})

	t.Run("service section all fields", func(t *testing.T) {
		remainAfterExit := true
		u := &ServiceUnit{
			Service: &ServiceSection{
				Type:             "simple",
				ExecStart:        "/usr/bin/myapp --config /etc/myapp.conf",
				ExecStartPre:     []string{"/usr/bin/myapp-check"},
				ExecStartPost:    []string{"/usr/bin/myapp-notify"},
				ExecStop:         "/usr/bin/myapp --stop",
				ExecStopPost:     []string{"/usr/bin/myapp-cleanup"},
				ExecReload:       "/bin/kill -HUP $MAINPID",
				Restart:          "on-failure",
				RestartSec:       "5",
				TimeoutStartSec:  "30",
				TimeoutStopSec:   "10",
				User:             "myapp",
				Group:            "myapp",
				WorkingDirectory: "/var/lib/myapp",
				EnvironmentFile:  "/etc/default/myapp",
				StandardOutput:   "journal",
				StandardError:    "journal",
				RemainAfterExit:  &remainAfterExit,
			},
		}

		got := u.Serialize()
		assert.Contains(t, got, "Type=simple\n")
		assert.Contains(t, got, "ExecStart=/usr/bin/myapp --config /etc/myapp.conf\n")
		assert.Contains(t, got, "ExecStartPre=/usr/bin/myapp-check\n")
		assert.Contains(t, got, "ExecStartPost=/usr/bin/myapp-notify\n")
		assert.Contains(t, got, "ExecStop=/usr/bin/myapp --stop\n")
		assert.Contains(t, got, "ExecStopPost=/usr/bin/myapp-cleanup\n")
		assert.Contains(t, got, "ExecReload=/bin/kill -HUP $MAINPID\n")
		assert.Contains(t, got, "Restart=on-failure\n")
		assert.Contains(t, got, "RestartSec=5\n")
		assert.Contains(t, got, "TimeoutStartSec=30\n")
		assert.Contains(t, got, "TimeoutStopSec=10\n")
		assert.Contains(t, got, "User=myapp\n")
		assert.Contains(t, got, "Group=myapp\n")
		assert.Contains(t, got, "WorkingDirectory=/var/lib/myapp\n")
		assert.Contains(t, got, "EnvironmentFile=/etc/default/myapp\n")
		assert.Contains(t, got, "StandardOutput=journal\n")
		assert.Contains(t, got, "StandardError=journal\n")
		assert.Contains(t, got, "RemainAfterExit=yes\n")
	})

	t.Run("remain after exit false", func(t *testing.T) {
		val := false
		u := &ServiceUnit{
			Service: &ServiceSection{
				ExecStart:       "/bin/true",
				RemainAfterExit: &val,
			},
		}

		got := u.Serialize()
		assert.Contains(t, got, "RemainAfterExit=no\n")
	})

	t.Run("remain after exit nil omitted", func(t *testing.T) {
		u := &ServiceUnit{
			Service: &ServiceSection{
				ExecStart: "/bin/true",
			},
		}

		got := u.Serialize()
		assert.NotContains(t, got, "RemainAfterExit")
	})

	t.Run("environment sorted", func(t *testing.T) {
		u := &ServiceUnit{
			Service: &ServiceSection{
				ExecStart: "/bin/true",
				Environment: map[string]string{
					"ZZZ_VAR": "last",
					"AAA_VAR": "first",
					"MMM_VAR": "middle",
				},
			},
		}

		got := u.Serialize()
		aIdx := strings.Index(got, `Environment="AAA_VAR=first"`)
		mIdx := strings.Index(got, `Environment="MMM_VAR=middle"`)
		zIdx := strings.Index(got, `Environment="ZZZ_VAR=last"`)
		assert.Greater(t, mIdx, aIdx, "MMM should come after AAA")
		assert.Greater(t, zIdx, mIdx, "ZZZ should come after MMM")
	})

	t.Run("extra directives sorted", func(t *testing.T) {
		u := &ServiceUnit{
			Service: &ServiceSection{
				ExecStart: "/bin/true",
				Extra: map[string]string{
					"LimitNOFILE": "65536",
					"LimitNPROC":  "4096",
				},
			},
		}

		got := u.Serialize()
		assert.Contains(t, got, "LimitNOFILE=65536\n")
		assert.Contains(t, got, "LimitNPROC=4096\n")
		nIdx := strings.Index(got, "LimitNOFILE")
		pIdx := strings.Index(got, "LimitNPROC")
		assert.Greater(t, pIdx, nIdx, "LimitNPROC should come after LimitNOFILE")
	})

	t.Run("install section", func(t *testing.T) {
		u := &ServiceUnit{
			Install: &InstallSection{
				WantedBy:   []string{"multi-user.target", "graphical.target"},
				RequiredBy: []string{"important.target"},
				Alias:      []string{"myapp-alt.service"},
				Extra: map[string]string{
					"Also": "helper.service",
				},
			},
		}

		got := u.Serialize()
		assert.Contains(t, got, "WantedBy=multi-user.target\n")
		assert.Contains(t, got, "WantedBy=graphical.target\n")
		assert.Contains(t, got, "RequiredBy=important.target\n")
		assert.Contains(t, got, "Alias=myapp-alt.service\n")
		assert.Contains(t, got, "Also=helper.service\n")
	})

	t.Run("empty sections omitted", func(t *testing.T) {
		u := &ServiceUnit{}
		got := u.Serialize()
		assert.Equal(t, "", got)
	})

	t.Run("empty string fields omitted", func(t *testing.T) {
		u := &ServiceUnit{
			Service: &ServiceSection{
				ExecStart: "/bin/true",
				Type:      "",
				User:      "",
			},
		}

		got := u.Serialize()
		assert.NotContains(t, got, "Type=")
		assert.NotContains(t, got, "User=\n")
		assert.Contains(t, got, "ExecStart=/bin/true\n")
	})
}

// --------------------------------------------------------------------
// Deserialization unit tests (no SSH required)
// --------------------------------------------------------------------

func TestParseServiceUnit(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		content := "[Unit]\nDescription=Test\n\n[Service]\nExecStart=/bin/true\n"
		u := ParseServiceUnit(content)

		require.NotNil(t, u.Unit)
		assert.Equal(t, "Test", u.Unit.Description)
		require.NotNil(t, u.Service)
		assert.Equal(t, "/bin/true", u.Service.ExecStart)
		assert.Nil(t, u.Install)
	})

	t.Run("all sections", func(t *testing.T) {
		content := "[Unit]\nDescription=Full\n\n[Service]\nExecStart=/bin/true\n\n[Install]\nWantedBy=multi-user.target\n"
		u := ParseServiceUnit(content)

		require.NotNil(t, u.Unit)
		require.NotNil(t, u.Service)
		require.NotNil(t, u.Install)
		assert.Equal(t, []string{"multi-user.target"}, u.Install.WantedBy)
	})

	t.Run("list directives", func(t *testing.T) {
		content := "[Unit]\nDescription=Lists\nAfter=network.target\nAfter=syslog.target\nWants=network-online.target\n"
		u := ParseServiceUnit(content)

		require.NotNil(t, u.Unit)
		assert.Equal(t, []string{"network.target", "syslog.target"}, u.Unit.After)
		assert.Equal(t, []string{"network-online.target"}, u.Unit.Wants)
	})

	t.Run("condition and assert", func(t *testing.T) {
		content := "[Unit]\nDescription=Checks\nConditionPathExists=/etc/app.conf\nConditionUser=root\nAssertFileNotEmpty=/etc/app.conf\nAssertHost=myhost\n"
		u := ParseServiceUnit(content)

		require.NotNil(t, u.Unit)
		require.NotNil(t, u.Unit.Condition)
		assert.Equal(t, []string{"/etc/app.conf"}, u.Unit.Condition.PathExists)
		assert.Equal(t, "root", u.Unit.Condition.User)
		require.NotNil(t, u.Unit.Assert)
		assert.Equal(t, []string{"/etc/app.conf"}, u.Unit.Assert.FileNotEmpty)
		assert.Equal(t, "myhost", u.Unit.Assert.Host)
	})

	t.Run("service all fields", func(t *testing.T) {
		content := strings.Join([]string{
			"[Service]",
			"Type=simple",
			"ExecStart=/usr/bin/app",
			"ExecStartPre=/usr/bin/check",
			"ExecStartPost=/usr/bin/notify",
			"ExecStop=/usr/bin/app --stop",
			"ExecStopPost=/usr/bin/cleanup",
			"ExecReload=/bin/kill -HUP $MAINPID",
			"Restart=on-failure",
			"RestartSec=5",
			"TimeoutStartSec=30",
			"TimeoutStopSec=10",
			"User=app",
			"Group=app",
			"WorkingDirectory=/var/lib/app",
			`Environment="FOO=bar"`,
			`Environment="BAZ=qux"`,
			"EnvironmentFile=/etc/default/app",
			"StandardOutput=journal",
			"StandardError=journal",
			"RemainAfterExit=yes",
			"LimitNOFILE=65536",
			"",
		}, "\n")

		u := ParseServiceUnit(content)
		require.NotNil(t, u.Service)
		s := u.Service

		assert.Equal(t, "simple", s.Type)
		assert.Equal(t, "/usr/bin/app", s.ExecStart)
		assert.Equal(t, []string{"/usr/bin/check"}, s.ExecStartPre)
		assert.Equal(t, []string{"/usr/bin/notify"}, s.ExecStartPost)
		assert.Equal(t, "/usr/bin/app --stop", s.ExecStop)
		assert.Equal(t, []string{"/usr/bin/cleanup"}, s.ExecStopPost)
		assert.Equal(t, "/bin/kill -HUP $MAINPID", s.ExecReload)
		assert.Equal(t, "on-failure", s.Restart)
		assert.Equal(t, "5", s.RestartSec)
		assert.Equal(t, "30", s.TimeoutStartSec)
		assert.Equal(t, "10", s.TimeoutStopSec)
		assert.Equal(t, "app", s.User)
		assert.Equal(t, "app", s.Group)
		assert.Equal(t, "/var/lib/app", s.WorkingDirectory)
		assert.Equal(t, map[string]string{"FOO": "bar", "BAZ": "qux"}, s.Environment)
		assert.Equal(t, "/etc/default/app", s.EnvironmentFile)
		assert.Equal(t, "journal", s.StandardOutput)
		assert.Equal(t, "journal", s.StandardError)
		require.NotNil(t, s.RemainAfterExit)
		assert.True(t, *s.RemainAfterExit)
		assert.Equal(t, map[string]string{"LimitNOFILE": "65536"}, s.Extra)
	})

	t.Run("remain after exit variants", func(t *testing.T) {
		for _, tc := range []struct {
			value string
			want  bool
		}{
			{"yes", true},
			{"true", true},
			{"1", true},
			{"on", true},
			{"no", false},
			{"false", false},
			{"0", false},
			{"off", false},
		} {
			t.Run(tc.value, func(t *testing.T) {
				content := fmt.Sprintf("[Service]\nExecStart=/bin/true\nRemainAfterExit=%s\n", tc.value)
				u := ParseServiceUnit(content)
				require.NotNil(t, u.Service)
				require.NotNil(t, u.Service.RemainAfterExit)
				assert.Equal(t, tc.want, *u.Service.RemainAfterExit)
			})
		}
	})

	t.Run("environment without quotes", func(t *testing.T) {
		content := "[Service]\nExecStart=/bin/true\nEnvironment=FOO=bar\n"
		u := ParseServiceUnit(content)
		require.NotNil(t, u.Service)
		assert.Equal(t, map[string]string{"FOO": "bar"}, u.Service.Environment)
	})

	t.Run("environment with equals in value", func(t *testing.T) {
		content := "[Service]\nExecStart=/bin/true\nEnvironment=\"OPTS=--flag=val\"\n"
		u := ParseServiceUnit(content)
		require.NotNil(t, u.Service)
		assert.Equal(t, map[string]string{"OPTS": "--flag=val"}, u.Service.Environment)
	})

	t.Run("install extra directives", func(t *testing.T) {
		content := "[Install]\nWantedBy=multi-user.target\nAlso=helper.service\n"
		u := ParseServiceUnit(content)
		require.NotNil(t, u.Install)
		assert.Equal(t, []string{"multi-user.target"}, u.Install.WantedBy)
		assert.Equal(t, map[string]string{"Also": "helper.service"}, u.Install.Extra)
	})

	t.Run("comments and blank lines ignored", func(t *testing.T) {
		content := "# This is a comment\n; Another comment\n\n[Unit]\nDescription=Test\n\n# inline comment\n[Service]\nExecStart=/bin/true\n"
		u := ParseServiceUnit(content)
		require.NotNil(t, u.Unit)
		assert.Equal(t, "Test", u.Unit.Description)
		require.NotNil(t, u.Service)
		assert.Equal(t, "/bin/true", u.Service.ExecStart)
	})

	t.Run("lines before first section ignored", func(t *testing.T) {
		content := "stray=line\n[Unit]\nDescription=Test\n"
		u := ParseServiceUnit(content)
		require.NotNil(t, u.Unit)
		assert.Equal(t, "Test", u.Unit.Description)
	})

	t.Run("empty content", func(t *testing.T) {
		u := ParseServiceUnit("")
		assert.Nil(t, u.Unit)
		assert.Nil(t, u.Service)
		assert.Nil(t, u.Install)
	})

	t.Run("unknown sections ignored", func(t *testing.T) {
		content := "[Unit]\nDescription=Test\n\n[Timer]\nOnCalendar=daily\n"
		u := ParseServiceUnit(content)
		require.NotNil(t, u.Unit)
		assert.Nil(t, u.Service)
		assert.Nil(t, u.Install)
	})
}

func TestSerializeDeserializeRoundtrip(t *testing.T) {
	t.Run("full unit", func(t *testing.T) {
		remainAfterExit := true
		original := &ServiceUnit{
			Unit: &UnitSection{
				Description: "Roundtrip test",
				After:       []string{"network.target"},
				Requires:    []string{"network.target"},
				Condition: &CheckDirectives{
					PathExists: []string{"/etc/app.conf"},
					User:       "root",
				},
			},
			Service: &ServiceSection{
				Type:            "simple",
				ExecStart:       "/usr/bin/app",
				ExecStartPre:    []string{"/usr/bin/check"},
				Restart:         "on-failure",
				RestartSec:      "5",
				User:            "app",
				Group:           "app",
				Environment:     map[string]string{"FOO": "bar", "BAZ": "qux"},
				RemainAfterExit: &remainAfterExit,
				Extra:           map[string]string{"LimitNOFILE": "65536"},
			},
			Install: &InstallSection{
				WantedBy: []string{"multi-user.target"},
			},
		}

		serialized := original.Serialize()
		parsed := ParseServiceUnit(serialized)

		// Re-serialize and compare — the two serializations should be identical.
		reserialized := parsed.Serialize()
		assert.Equal(t, serialized, reserialized)

		// Spot check fields
		require.NotNil(t, parsed.Unit)
		assert.Equal(t, "Roundtrip test", parsed.Unit.Description)
		assert.Equal(t, []string{"network.target"}, parsed.Unit.After)
		require.NotNil(t, parsed.Unit.Condition)
		assert.Equal(t, []string{"/etc/app.conf"}, parsed.Unit.Condition.PathExists)
		assert.Equal(t, "root", parsed.Unit.Condition.User)

		require.NotNil(t, parsed.Service)
		assert.Equal(t, "simple", parsed.Service.Type)
		assert.Equal(t, map[string]string{"FOO": "bar", "BAZ": "qux"}, parsed.Service.Environment)
		require.NotNil(t, parsed.Service.RemainAfterExit)
		assert.True(t, *parsed.Service.RemainAfterExit)
		assert.Equal(t, map[string]string{"LimitNOFILE": "65536"}, parsed.Service.Extra)

		require.NotNil(t, parsed.Install)
		assert.Equal(t, []string{"multi-user.target"}, parsed.Install.WantedBy)
	})

	t.Run("minimal unit", func(t *testing.T) {
		original := &ServiceUnit{
			Service: &ServiceSection{
				ExecStart: "/bin/true",
			},
		}

		serialized := original.Serialize()
		parsed := ParseServiceUnit(serialized)
		reserialized := parsed.Serialize()
		assert.Equal(t, serialized, reserialized)
	})
}

// --------------------------------------------------------------------
// Integration tests (require SSH to a VM)
// --------------------------------------------------------------------

func TestWriteServiceUnit(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("writes and reads back", func(t *testing.T) {
		name := "tf-test-write"
		t.Cleanup(func() {
			client.DeleteServiceUnit(ctx, name)
			client.DaemonReload(ctx)
		})

		remainAfterExit := true
		original := &ServiceUnit{
			Unit: &UnitSection{
				Description: "Write test service",
				After:       []string{"network.target"},
			},
			Service: &ServiceSection{
				Type:            "oneshot",
				ExecStart:       "/bin/true",
				RemainAfterExit: &remainAfterExit,
			},
			Install: &InstallSection{
				WantedBy: []string{"multi-user.target"},
			},
		}

		err := client.WriteServiceUnit(ctx, name, original)
		require.NoError(t, err)

		// Verify file exists with correct permissions
		filePath := "/etc/systemd/system/" + name + ".service"
		gotMode := hostRun(t, client, fmt.Sprintf("stat -c '%%a' %s", filePath))
		assert.Equal(t, "644", gotMode)

		// Read back and verify content
		got, err := client.ReadServiceUnit(ctx, name)
		require.NoError(t, err)

		assert.Equal(t, "Write test service", got.Unit.Description)
		assert.Equal(t, []string{"network.target"}, got.Unit.After)
		assert.Equal(t, "oneshot", got.Service.Type)
		assert.Equal(t, "/bin/true", got.Service.ExecStart)
		require.NotNil(t, got.Service.RemainAfterExit)
		assert.True(t, *got.Service.RemainAfterExit)
		assert.Equal(t, []string{"multi-user.target"}, got.Install.WantedBy)
	})

	t.Run("overwrites existing", func(t *testing.T) {
		name := "tf-test-overwrite"
		t.Cleanup(func() {
			client.DeleteServiceUnit(ctx, name)
			client.DaemonReload(ctx)
		})

		first := &ServiceUnit{
			Service: &ServiceSection{
				Type:      "oneshot",
				ExecStart: "/bin/true",
			},
		}
		err := client.WriteServiceUnit(ctx, name, first)
		require.NoError(t, err)

		second := &ServiceUnit{
			Service: &ServiceSection{
				Type:      "oneshot",
				ExecStart: "/bin/false",
			},
		}
		err = client.WriteServiceUnit(ctx, name, second)
		require.NoError(t, err)

		got, err := client.ReadServiceUnit(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, "/bin/false", got.Service.ExecStart)
	})
}

func TestReadServiceUnit(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("not found", func(t *testing.T) {
		_, err := client.ReadServiceUnit(ctx, "tf-test-nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestServiceUnitExists(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("returns false for nonexistent", func(t *testing.T) {
		exists, err := client.ServiceUnitExists(ctx, "tf-test-no-such-unit")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("returns true for existing", func(t *testing.T) {
		name := "tf-test-exists"
		t.Cleanup(func() {
			client.DeleteServiceUnit(ctx, name)
			client.DaemonReload(ctx)
		})

		err := client.WriteServiceUnit(ctx, name, &ServiceUnit{
			Service: &ServiceSection{
				Type:      "oneshot",
				ExecStart: "/bin/true",
			},
		})
		require.NoError(t, err)

		exists, err := client.ServiceUnitExists(ctx, name)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestDeleteServiceUnit(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("removes existing", func(t *testing.T) {
		name := "tf-test-delete"

		err := client.WriteServiceUnit(ctx, name, &ServiceUnit{
			Service: &ServiceSection{
				Type:      "oneshot",
				ExecStart: "/bin/true",
			},
		})
		require.NoError(t, err)

		err = client.DeleteServiceUnit(ctx, name)
		require.NoError(t, err)

		exists, err := client.ServiceUnitExists(ctx, name)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("nonexistent returns error", func(t *testing.T) {
		err := client.DeleteServiceUnit(ctx, "tf-test-delete-nonexistent")
		assert.Error(t, err)
	})
}

func TestGetServiceState(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	t.Run("unknown service", func(t *testing.T) {
		state, err := client.GetServiceState(ctx, "tf-test-unknown-svc")
		require.NoError(t, err)
		assert.False(t, state.Enabled)
		assert.False(t, state.Active)
	})

	t.Run("enabled and active", func(t *testing.T) {
		name := "tf-test-state"
		remainAfterExit := true
		t.Cleanup(func() {
			client.StopService(ctx, name)
			client.DisableService(ctx, name)
			client.DeleteServiceUnit(ctx, name)
			client.DaemonReload(ctx)
		})

		err := client.WriteServiceUnit(ctx, name, &ServiceUnit{
			Unit: &UnitSection{
				Description: "State test",
			},
			Service: &ServiceSection{
				Type:            "oneshot",
				ExecStart:       "/bin/true",
				RemainAfterExit: &remainAfterExit,
			},
			Install: &InstallSection{
				WantedBy: []string{"multi-user.target"},
			},
		})
		require.NoError(t, err)
		require.NoError(t, client.DaemonReload(ctx))

		// Initially disabled and inactive
		state, err := client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.False(t, state.Enabled)
		assert.False(t, state.Active)

		// Enable
		require.NoError(t, client.EnableService(ctx, name))
		state, err = client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.True(t, state.Enabled)
		assert.False(t, state.Active)

		// Start
		require.NoError(t, client.StartService(ctx, name))
		state, err = client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.True(t, state.Enabled)
		assert.True(t, state.Active)

		// Stop
		require.NoError(t, client.StopService(ctx, name))
		state, err = client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.True(t, state.Enabled)
		assert.False(t, state.Active)

		// Disable
		require.NoError(t, client.DisableService(ctx, name))
		state, err = client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.False(t, state.Enabled)
		assert.False(t, state.Active)
	})
}

func TestEnableDisableService(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	name := "tf-test-enable"
	t.Cleanup(func() {
		client.DisableService(ctx, name)
		client.DeleteServiceUnit(ctx, name)
		client.DaemonReload(ctx)
	})

	err := client.WriteServiceUnit(ctx, name, &ServiceUnit{
		Unit: &UnitSection{
			Description: "Enable test",
		},
		Service: &ServiceSection{
			Type:      "oneshot",
			ExecStart: "/bin/true",
		},
		Install: &InstallSection{
			WantedBy: []string{"multi-user.target"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, client.DaemonReload(ctx))

	t.Run("enable", func(t *testing.T) {
		err := client.EnableService(ctx, name)
		require.NoError(t, err)

		state, err := client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.True(t, state.Enabled)
	})

	t.Run("disable", func(t *testing.T) {
		err := client.DisableService(ctx, name)
		require.NoError(t, err)

		state, err := client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.False(t, state.Enabled)
	})
}

func TestStartStopRestartService(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	name := "tf-test-startstop"
	remainAfterExit := true
	t.Cleanup(func() {
		client.StopService(ctx, name)
		client.DeleteServiceUnit(ctx, name)
		client.DaemonReload(ctx)
	})

	err := client.WriteServiceUnit(ctx, name, &ServiceUnit{
		Unit: &UnitSection{
			Description: "Start/stop test",
		},
		Service: &ServiceSection{
			Type:            "oneshot",
			ExecStart:       "/bin/true",
			RemainAfterExit: &remainAfterExit,
		},
	})
	require.NoError(t, err)
	require.NoError(t, client.DaemonReload(ctx))

	t.Run("start", func(t *testing.T) {
		err := client.StartService(ctx, name)
		require.NoError(t, err)

		state, err := client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.True(t, state.Active)
	})

	t.Run("restart", func(t *testing.T) {
		err := client.RestartService(ctx, name)
		require.NoError(t, err)

		state, err := client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.True(t, state.Active)
	})

	t.Run("stop", func(t *testing.T) {
		err := client.StopService(ctx, name)
		require.NoError(t, err)

		state, err := client.GetServiceState(ctx, name)
		require.NoError(t, err)
		assert.False(t, state.Active)
	})
}

func TestDaemonReload(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	err := client.DaemonReload(ctx)
	require.NoError(t, err)
}
