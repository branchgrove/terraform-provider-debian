package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccSystemdServiceResource_customBasic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdServiceCustomConfig("tf-acc-basic", "/bin/true", false, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-basic")),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("active"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

func TestAccSystemdServiceResource_customEnabledActive(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdServiceOneshotConfig("tf-acc-active", true, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("active"), knownvalue.Bool(true)),
				},
			},
		},
	})
}

func TestAccSystemdServiceResource_customUpdate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdServiceCustomConfig("tf-acc-update", "/bin/true", false, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(false)),
				},
			},
			{
				Config: testAccSystemdServiceOneshotConfig("tf-acc-update", true, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("active"), knownvalue.Bool(true)),
				},
			},
		},
	})
}

func TestAccSystemdServiceResource_customFull(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdServiceFullConfig("tf-acc-full"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-full")),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("service").AtMapKey("restart"), knownvalue.StringExact("on-failure")),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("service").AtMapKey("restart_sec"), knownvalue.StringExact("5")),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("install").AtMapKey("wanted_by"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("multi-user.target")})),
				},
			},
		},
	})
}

func TestAccSystemdServiceResource_importState(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdServiceOneshotConfig("tf-acc-import", true, true),
			},
			{
				ResourceName:                         "debian_systemd_service.test",
				ImportState:                          true,
				ImportStateId:                        testImportID("tf-acc-import"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"ssh.private_key", "ssh.public_key", "ssh.password", "ssh.host_key"},
			},
		},
	})
}

// --- package-installed service tests ---

func TestAccSystemdServiceResource_packageInstalled(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdServicePackageConfig("cron", true, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("name"), knownvalue.StringExact("cron")),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("active"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("unit"), knownvalue.Null()),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("service"), knownvalue.Null()),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("install"), knownvalue.Null()),
				},
			},
		},
	})
}

func TestAccSystemdServiceResource_packageInstalledDisable(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Start enabled and active
			{
				Config: testAccSystemdServicePackageConfig("cron", true, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("active"), knownvalue.Bool(true)),
				},
			},
			// Disable and stop
			{
				Config: testAccSystemdServicePackageConfig("cron", false, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("active"), knownvalue.Bool(false)),
				},
			},
			// Re-enable and restart
			{
				Config: testAccSystemdServicePackageConfig("cron", true, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("debian_systemd_service.test",
						tfjsonpath.New("active"), knownvalue.Bool(true)),
				},
			},
		},
	})
}

func TestAccSystemdServiceResource_packageInstalledImport(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemdServicePackageConfig("cron", true, true),
			},
			{
				ResourceName:                         "debian_systemd_service.test",
				ImportState:                          true,
				ImportStateId:                        testImportID("cron"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"ssh.private_key", "ssh.public_key", "ssh.password", "ssh.host_key"},
			},
		},
	})
}

// --- config helpers ---

func testAccSystemdServiceCustomConfig(name, execStart string, enabled, active bool) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_systemd_service" "test" {
  name    = %[1]q
  enabled = %[2]t
  active  = %[3]t

  unit = {
    description = "Test service %[1]s"
  }

  service = {
    type       = "oneshot"
    exec_start = %[4]q
    remain_after_exit = true
  }

  install = {
    wanted_by = ["multi-user.target"]
  }

%[5]s
}
`, name, enabled, active, execStart, testSSHBlock())
}

func testAccSystemdServiceOneshotConfig(name string, enabled, active bool) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_systemd_service" "test" {
  name    = %[1]q
  enabled = %[2]t
  active  = %[3]t

  unit = {
    description = "Test service %[1]s"
  }

  service = {
    type              = "oneshot"
    exec_start        = "/bin/true"
    remain_after_exit = true
  }

  install = {
    wanted_by = ["multi-user.target"]
  }

%[4]s
}
`, name, enabled, active, testSSHBlock())
}

func testAccSystemdServicePackageConfig(name string, enabled, active bool) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_apt_packages" "test" {
  packages = {
    %[1]q = ""
  }

%[4]s
}

resource "debian_systemd_service" "test" {
  depends_on = [debian_apt_packages.test]

  name    = %[1]q
  enabled = %[2]t
  active  = %[3]t

%[4]s
}
`, name, enabled, active, testSSHBlock())
}

func testAccSystemdServiceFullConfig(name string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_systemd_service" "test" {
  name    = %[1]q
  enabled = false
  active  = false

  unit = {
    description = "Full test service"
    after       = ["network.target"]
    requires    = ["network.target"]
  }

  service = {
    type              = "oneshot"
    exec_start        = "/bin/true"
    restart           = "on-failure"
    restart_sec       = "5"
    remain_after_exit = true

    extra = {
      "LimitNOFILE" = "65536"
    }
  }

  install = {
    wanted_by = ["multi-user.target"]
  }

%[2]s
}
`, name, testSSHBlock())
}
