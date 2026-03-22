package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAptPackagesResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock() + `
resource "debian_apt_packages" "test" {
  packages = {
    "sl" = ""
  }
` + testSSHBlock() + `
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_apt_packages.test",
						tfjsonpath.New("installed_packages").AtMapKey("sl"),
						knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestAccAptPackagesResource_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock() + `
resource "debian_apt_packages" "test" {
  packages = {
    "sl" = ""
  }
` + testSSHBlock() + `
}
`,
			},
			{
				Config: testProviderBlock() + `
resource "debian_apt_packages" "test" {
  packages = {
    "sl"    = ""
    "cowsay" = ""
  }
` + testSSHBlock() + `
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_apt_packages.test",
						tfjsonpath.New("installed_packages").AtMapKey("sl"),
						knownvalue.NotNull()),
					statecheck.ExpectKnownValue("debian_apt_packages.test",
						tfjsonpath.New("installed_packages").AtMapKey("cowsay"),
						knownvalue.NotNull()),
				},
			},
		},
	})
}
