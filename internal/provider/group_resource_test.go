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

func TestAccGroupResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupResourceConfig("tfgroup1", false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_group.test",
						tfjsonpath.New("name"), knownvalue.StringExact("tfgroup1")),
					statecheck.ExpectKnownValue("debian_group.test",
						tfjsonpath.New("system"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

func TestAccGroupResource_system(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupResourceConfig("tfsysgrp1", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_group.test",
						tfjsonpath.New("name"), knownvalue.StringExact("tfsysgrp1")),
					statecheck.ExpectKnownValue("debian_group.test",
						tfjsonpath.New("system"), knownvalue.Bool(true)),
				},
			},
		},
	})
}

func TestAccGroupResource_importState(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupResourceConfig("tfgroup2", false),
			},
			{
				ResourceName:                         "debian_group.test",
				ImportState:                          true,
				ImportStateId:                        testImportID("tfgroup2"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"system", "overwrite"},
			},
		},
	})
}

func TestAccGroupResource_users(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupResourceWithUsers("tfgroup3", []string{}),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_group.test",
						tfjsonpath.New("name"), knownvalue.StringExact("tfgroup3")),
				},
			},
		},
	})
}

// --- config helpers ---

func testAccGroupResourceConfig(name string, system bool) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_group" "test" {
  name   = %[1]q
  system = %[2]t
%[3]s
}
`, name, system, testSSHBlock())
}

func testAccGroupResourceWithUsers(name string, users []string) string {
	usersStr := "[]"
	if len(users) > 0 {
		quoted := make([]string, len(users))
		for i, u := range users {
			quoted[i] = fmt.Sprintf("%q", u)
		}
		usersStr = fmt.Sprintf("[%s]", joinStrings(quoted))
	}

	return testProviderBlock() + fmt.Sprintf(`
resource "debian_group" "test" {
  name  = %[1]q
  users = %[2]s
%[3]s
}
`, name, usersStr, testSSHBlock())
}

func joinStrings(s []string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += v
	}
	return result
}

func TestAccGroupResource_overwrite(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccGroupConfigOverwrite("tfgroup_ow", false),
				ExpectError: regexp.MustCompile("Resource already exists"),
			},
			{
				Config: testAccGroupConfigOverwrite("tfgroup_ow", true),
			},
		},
	})
}

func testAccGroupConfigOverwrite(name string, overwrite bool) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_group" "setup" {
  name = %[1]q
%[3]s
}

resource "debian_group" "test" {
  name       = %[1]q
  overwrite  = %[2]t
  depends_on = [debian_group.setup]
%[3]s
}
`, name, overwrite, testSSHBlock())
}
