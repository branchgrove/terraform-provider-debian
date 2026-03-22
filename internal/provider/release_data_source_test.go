package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccReleaseDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccReleaseDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.debian_release.test",
						tfjsonpath.New("id"), knownvalue.StringExact("debian")),
					statecheck.ExpectKnownValue("data.debian_release.test",
						tfjsonpath.New("version_id"), knownvalue.StringExact("12")),
					statecheck.ExpectKnownValue("data.debian_release.test",
						tfjsonpath.New("version_codename"), knownvalue.StringExact("bookworm")),
				},
			},
		},
	})
}

func testAccReleaseDataSourceConfig() string {
	return testProviderBlock() + fmt.Sprintf(`
data "debian_release" "test" {
%s
}
`, testSSHBlock())
}
