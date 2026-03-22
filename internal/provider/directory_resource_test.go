package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccDirectoryResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDirectoryConfig("/tmp/tf_acc_dir_basic", "0755"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("path"), knownvalue.StringExact("/tmp/tf_acc_dir_basic")),
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("mode"), knownvalue.StringExact("0755")),
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("basename"), knownvalue.StringExact("tf_acc_dir_basic")),
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("dirname"), knownvalue.StringExact("/tmp")),
				},
			},
		},
	})
}

func TestAccDirectoryResource_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDirectoryConfig("/tmp/tf_acc_dir_update", "0755"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("mode"), knownvalue.StringExact("0755")),
				},
			},
			{
				Config: testAccDirectoryConfig("/tmp/tf_acc_dir_update", "0700"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("mode"), knownvalue.StringExact("0700")),
				},
			},
		},
	})
}

func TestAccDirectoryResource_importState(t *testing.T) {
	dirPath := "/tmp/tf_acc_dir_import"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDirectoryConfig(dirPath, "0755"),
			},
			{
				ResourceName:                         "debian_directory.test",
				ImportState:                          true,
				ImportStateId:                        testImportID(dirPath),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "path",
				ImportStateVerifyIgnore:              []string{"create_parents", "ssh.private_key", "ssh.public_key", "ssh.password", "ssh.host_key"},
			},
		},
	})
}

func TestAccDirectoryResource_importStateWithKey(t *testing.T) {
	dirPath := "/tmp/tf_acc_dir_import_key"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDirectoryConfig(dirPath, "0755"),
			},
			{
				ResourceName:                         "debian_directory.test",
				ImportState:                          true,
				ImportStateId:                        testImportIDWithKey(dirPath),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "path",
				ImportStateVerifyIgnore:              []string{"create_parents", "ssh.private_key", "ssh.public_key", "ssh.password", "ssh.host_key"},
			},
		},
	})
}

func TestAccDirectoryResource_owner(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDirectoryConfigWithOwner("/tmp/tf_acc_dir_owner", "0755", "nobody", "nogroup"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("owner"), knownvalue.StringExact("nobody")),
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("group"), knownvalue.StringExact("nogroup")),
				},
			},
		},
	})
}

func TestAccDirectoryResource_createParents(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDirectoryConfigWithParents("/tmp/tf_acc_dir_nested/sub/dir", "0755"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("basename"), knownvalue.StringExact("dir")),
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("dirname"), knownvalue.StringExact("/tmp/tf_acc_dir_nested/sub")),
				},
			},
		},
	})
}

func TestAccDirectoryResource_modeDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDirectoryConfigNoMode("/tmp/tf_acc_dir_default_mode"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_directory.test",
						tfjsonpath.New("mode"), knownvalue.StringExact("0755")),
				},
			},
		},
	})
}

// --- config helpers ---

func testAccDirectoryConfig(path, mode string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_directory" "test" {
  path = %[1]q
  mode = %[2]q
%[3]s
}
`, path, mode, testSSHBlock())
}

func testAccDirectoryConfigNoMode(path string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_directory" "test" {
  path = %[1]q
%[2]s
}
`, path, testSSHBlock())
}

func testAccDirectoryConfigWithOwner(path, mode, owner, group string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_directory" "test" {
  path  = %[1]q
  mode  = %[2]q
  owner = %[3]q
  group = %[4]q
%[5]s
}
`, path, mode, owner, group, testSSHBlock())
}

func testAccDirectoryConfigWithParents(path, mode string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_directory" "test" {
  path           = %[1]q
  mode           = %[2]q
  create_parents = true
%[3]s
}
`, path, mode, testSSHBlock())
}
