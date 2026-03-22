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

func TestAccCommandDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCommandDataSourceConfig("echo -n hello"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.debian_command.test",
						tfjsonpath.New("stdout"), knownvalue.StringExact("hello")),
					statecheck.ExpectKnownValue("data.debian_command.test",
						tfjsonpath.New("exit_code"), knownvalue.Int64Exact(0)),
				},
			},
		},
	})
}

func TestAccCommandDataSource_env(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCommandDataSourceConfigWithEnv(`echo -n "$MY_VAR"`, "MY_VAR", "test_value"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.debian_command.test",
						tfjsonpath.New("stdout"), knownvalue.StringExact("test_value")),
				},
			},
		},
	})
}

func TestAccCommandDataSource_stdin(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCommandDataSourceConfigWithStdin("cat", "hello from stdin"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.debian_command.test",
						tfjsonpath.New("stdout"), knownvalue.StringExact("hello from stdin")),
				},
			},
		},
	})
}

func TestAccCommandDataSource_allowError(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCommandDataSourceConfigAllowError("exit 42"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.debian_command.test",
						tfjsonpath.New("exit_code"), knownvalue.Int64Exact(42)),
				},
			},
		},
	})
}

func TestAccCommandDataSource_errorWithoutAllow(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCommandDataSourceConfig("exit 1"),
				ExpectError: regexp.MustCompile(`Command failed`),
			},
		},
	})
}

func TestAccCommandDataSource_stderr(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCommandDataSourceConfigAllowError("echo -n oops >&2; exit 1"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.debian_command.test",
						tfjsonpath.New("stderr"), knownvalue.StringExact("oops")),
					statecheck.ExpectKnownValue("data.debian_command.test",
						tfjsonpath.New("exit_code"), knownvalue.Int64Exact(1)),
				},
			},
		},
	})
}

// --- config helpers ---

func testAccCommandDataSourceConfig(command string) string {
	return testProviderBlock() + fmt.Sprintf(`
data "debian_command" "test" {
  command = %[1]q
%[2]s
}
`, command, testSSHBlock())
}

func testAccCommandDataSourceConfigWithEnv(command, key, value string) string {
	return testProviderBlock() + fmt.Sprintf(`
data "debian_command" "test" {
  command = %[1]q
  env = {
    %[2]q = %[3]q
  }
%[4]s
}
`, command, key, value, testSSHBlock())
}

func testAccCommandDataSourceConfigWithStdin(command, stdin string) string {
	return testProviderBlock() + fmt.Sprintf(`
data "debian_command" "test" {
  command = %[1]q
  stdin   = %[2]q
%[3]s
}
`, command, stdin, testSSHBlock())
}

func testAccCommandDataSourceConfigAllowError(command string) string {
	return testProviderBlock() + fmt.Sprintf(`
data "debian_command" "test" {
  command     = %[1]q
  allow_error = true
%[2]s
}
`, command, testSSHBlock())
}
