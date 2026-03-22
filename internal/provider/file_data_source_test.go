package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccFileDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileDataSourceConfig("/etc/hostname"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.debian_file.test",
						tfjsonpath.New("path"), knownvalue.StringExact("/etc/hostname")),
					statecheck.ExpectKnownValue("data.debian_file.test",
						tfjsonpath.New("basename"), knownvalue.StringExact("hostname")),
					statecheck.ExpectKnownValue("data.debian_file.test",
						tfjsonpath.New("dirname"), knownvalue.StringExact("/etc")),
					statecheck.ExpectKnownValue("data.debian_file.test",
						tfjsonpath.New("mode"), knownvalue.StringExact("0644")),
				},
			},
		},
	})
}

func TestAccFileDataSource_content(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileDataSourceWithCreate("/tmp/tf-file-ds-test", "hello data source"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.debian_file.test",
						tfjsonpath.New("content"), knownvalue.StringExact("hello data source")),
				},
			},
		},
	})
}

func testAccFileDataSourceConfig(path string) string {
	return testProviderBlock() + fmt.Sprintf(`
data "debian_file" "test" {
  path = %[1]q
%[2]s
}
`, path, testSSHBlock())
}

func testAccFileDataSourceWithCreate(path, content string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_file" "setup" {
  path    = %[1]q
  content = %[2]q
%[3]s
}

data "debian_file" "test" {
  path = debian_file.setup.path
%[3]s
}
`, path, content, testSSHBlock())
}
