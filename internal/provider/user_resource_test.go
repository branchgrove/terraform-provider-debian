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

func TestAccUserResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceConfig("tfuser1"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_user.test",
						tfjsonpath.New("name"), knownvalue.StringExact("tfuser1")),
					statecheck.ExpectKnownValue("debian_user.test",
						tfjsonpath.New("system"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

func TestAccUserResource_shell(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceWithShell("tfuser2", "/bin/sh"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_user.test",
						tfjsonpath.New("shell"), knownvalue.StringExact("/bin/sh")),
				},
			},
			{
				Config: testAccUserResourceWithShell("tfuser2", "/bin/bash"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_user.test",
						tfjsonpath.New("shell"), knownvalue.StringExact("/bin/bash")),
				},
			},
		},
	})
}

func TestAccUserResource_importState(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceConfig("tfuser3"),
			},
			{
				ResourceName:                         "debian_user.test",
				ImportState:                          true,
				ImportStateId:                        testImportID("tfuser3"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"system", "create_home", "overwrite"},
			},
		},
	})
}

func TestAccUserResource_system(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceSystem("tfsysuser1"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_user.test",
						tfjsonpath.New("system"), knownvalue.Bool(true)),
				},
			},
		},
	})
}

// --- config helpers ---

func testAccUserResourceConfig(name string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_user" "test" {
  name = %[1]q
%[2]s
}
`, name, testSSHBlock())
}

func testAccUserResourceWithShell(name, shell string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_user" "test" {
  name  = %[1]q
  shell = %[2]q
%[3]s
}
`, name, shell, testSSHBlock())
}

func testAccUserResourceSystem(name string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_user" "test" {
  name        = %[1]q
  system      = true
  create_home = false
%[2]s
}
`, name, testSSHBlock())
}

func TestAccUserResource_overwrite(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserConfigOverwrite("tfuser_ow", false),
				ExpectError: regexp.MustCompile("Resource already exists"),
			},
			{
				Config: testAccUserConfigOverwrite("tfuser_ow", true),
			},
		},
	})
}

func testAccUserConfigOverwrite(name string, overwrite bool) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_user" "setup" {
  name = %[1]q
%[3]s
}

resource "debian_user" "test" {
  name       = %[1]q
  overwrite  = %[2]t
  depends_on = [debian_user.setup]
%[3]s
}
`, name, overwrite, testSSHBlock())
}
