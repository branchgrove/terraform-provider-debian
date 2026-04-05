package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccSystemdTimerResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdTimerConfig("tf-acc-timer", "daily", false, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-timer")),
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("active"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

func TestAccSystemdTimerResource_enabledActive(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdTimerConfig("tf-acc-timer-active", "daily", true, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("active"), knownvalue.Bool(true)),
				},
			},
		},
	})
}

func TestAccSystemdTimerResource_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdTimerConfig("tf-acc-timer-update", "daily", false, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(false)),
				},
			},
			{
				Config: testAccSystemdTimerConfig("tf-acc-timer-update", "weekly", true, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("active"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("timer").AtMapKey("on_calendar"), knownvalue.StringExact("weekly")),
				},
			},
		},
	})
}

func TestAccSystemdTimerResource_full(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdTimerFullConfig("tf-acc-timer-full"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-timer-full")),
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("timer").AtMapKey("persistent"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_timer.test",
						tfjsonpath.New("install").AtMapKey("wanted_by"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("timers.target")})),
				},
			},
		},
	})
}

func TestAccSystemdTimerResource_importState(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdTimerConfig("tf-acc-timer-import", "daily", true, true),
			},
			{
				ResourceName:                         "debian_systemd_timer.test",
				ImportState:                          true,
				ImportStateId:                        testImportID("tf-acc-timer-import"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"overwrite", "ssh.private_key", "ssh.public_key", "ssh.password", "ssh.host_key", "active_timeout"},
			},
		},
	})
}

// --- config helpers ---

func testAccSystemdTimerConfig(name, onCalendar string, enabled, active bool) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_systemd_service" "test" {
  name    = %[1]q
  enabled = false
  active  = false

  service = {
    type       = "oneshot"
    exec_start = "/bin/true"
  }

%[5]s
}

resource "debian_systemd_timer" "test" {
  name    = %[1]q
  enabled = %[3]t
  active  = %[4]t

  unit = {
    description = "Test timer %[1]s"
  }

  timer = {
    on_calendar = %[2]q
  }

  install = {
    wanted_by = ["timers.target"]
  }

%[5]s
}
`, name, onCalendar, enabled, active, testSSHBlock())
}

func testAccSystemdTimerFullConfig(name string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_systemd_service" "test" {
  name    = %[1]q
  enabled = false
  active  = false

  service = {
    type       = "oneshot"
    exec_start = "/bin/true"
  }

%[2]s
}

resource "debian_systemd_timer" "test" {
  name    = %[1]q
  enabled = false
  active  = false

  unit = {
    description = "Full test timer"
  }

  timer = {
    on_calendar  = "daily"
    accuracy_sec = "1h"
    persistent   = true

    extra = {
      "FixedRandomDelay" = "true"
    }
  }

  install = {
    wanted_by = ["timers.target"]
  }

%[2]s
}
`, name, testSSHBlock())
}

func TestAccSystemdTimerResource_overwrite(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSystemdTimerConfigOverwrite("tf-acc-timer-ow", false),
				ExpectError: regexp.MustCompile("Resource already exists"),
			},
			{
				Config: testAccSystemdTimerConfigOverwrite("tf-acc-timer-ow", true),
			},
		},
	})
}

func testAccSystemdTimerConfigOverwrite(name string, overwrite bool) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_systemd_service" "setup" {
  name    = %[1]q
  enabled = false
  active  = false
  service = {
    type       = "oneshot"
    exec_start = "/bin/true"
  }
%[3]s
}

resource "debian_systemd_timer" "setup" {
  name       = %[1]q
  enabled    = false
  active     = false
  depends_on = [debian_systemd_service.setup]
  timer = {
    on_calendar = "daily"
  }
%[3]s
}

resource "debian_systemd_timer" "test" {
  name       = %[1]q
  enabled    = false
  active     = false
  overwrite  = %[2]t
  depends_on = [debian_systemd_timer.setup]
  timer = {
    on_calendar = "daily"
  }
%[3]s
}
`, name, overwrite, testSSHBlock())
}
