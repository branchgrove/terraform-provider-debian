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

func TestAccFileResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfig("/tmp/tf_acc_basic", "hello world", "0644"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("path"), knownvalue.StringExact("/tmp/tf_acc_basic")),
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("content"), knownvalue.StringExact("hello world")),
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("mode"), knownvalue.StringExact("0644")),
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("basename"), knownvalue.StringExact("tf_acc_basic")),
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("dirname"), knownvalue.StringExact("/tmp")),
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("size"), knownvalue.Int64Exact(11)),
				},
			},
		},
	})
}

func TestAccFileResource_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfig("/tmp/tf_acc_update", "version one", "0644"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("content"), knownvalue.StringExact("version one")),
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("mode"), knownvalue.StringExact("0644")),
				},
			},
			{
				Config: testAccFileConfig("/tmp/tf_acc_update", "version two", "0600"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("content"), knownvalue.StringExact("version two")),
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("mode"), knownvalue.StringExact("0600")),
				},
			},
		},
	})
}

func TestAccFileResource_importState(t *testing.T) {
	filePath := "/tmp/tf_acc_import"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfig(filePath, "import me", "0644"),
			},
			{
				ResourceName:                         "debian_file.test",
				ImportState:                          true,
				ImportStateId:                        testImportID(filePath),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "path",
				ImportStateVerifyIgnore:              []string{"content", "create_directories", "overwrite", "ssh.private_key", "ssh.public_key", "ssh.password", "ssh.host_key"},
			},
		},
	})
}

func TestAccFileResource_importStateWithKey(t *testing.T) {
	filePath := "/tmp/tf_acc_import_key"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfig(filePath, "import with key", "0644"),
			},
			{
				ResourceName:                         "debian_file.test",
				ImportState:                          true,
				ImportStateId:                        testImportIDWithKey(filePath),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "path",
				ImportStateVerifyIgnore:              []string{"content", "create_directories", "overwrite", "ssh.private_key", "ssh.public_key", "ssh.password", "ssh.host_key"},
			},
		},
	})
}

func TestAccFileResource_publicKey(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfigWithPublicKey("/tmp/tf_acc_pubkey", "public key auth", "0644"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("content"), knownvalue.StringExact("public key auth")),
				},
			},
		},
	})
}

func TestAccFileResource_owner(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfigWithOwner("/tmp/tf_acc_owner", "owned", "0644", "nobody", "nogroup"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("owner"), knownvalue.StringExact("nobody")),
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("group"), knownvalue.StringExact("nogroup")),
				},
			},
		},
	})
}

func TestAccFileResource_createDirectories(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfigWithDirs("/tmp/tf_acc_nested/sub/dir/file.txt", "nested content", "0644"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("basename"), knownvalue.StringExact("file.txt")),
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("dirname"), knownvalue.StringExact("/tmp/tf_acc_nested/sub/dir")),
				},
			},
		},
	})
}

func TestAccFileResource_modeDefault(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFileConfigNoMode("/tmp/tf_acc_default_mode", "default mode"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_file.test",
						tfjsonpath.New("mode"), knownvalue.StringExact("0644")),
				},
			},
		},
	})
}

// --- config helpers ---

func testAccFileConfig(path, content, mode string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_file" "test" {
  path    = %[1]q
  content = %[2]q
  mode    = %[3]q
%[4]s
}
`, path, content, mode, testSSHBlock())
}

func testAccFileConfigNoMode(path, content string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_file" "test" {
  path    = %[1]q
  content = %[2]q
%[3]s
}
`, path, content, testSSHBlock())
}

func testAccFileConfigWithOwner(path, content, mode, owner, group string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_file" "test" {
  path    = %[1]q
  content = %[2]q
  mode    = %[3]q
  owner   = %[4]q
  group   = %[5]q
%[6]s
}
`, path, content, mode, owner, group, testSSHBlock())
}

func testAccFileConfigWithDirs(path, content, mode string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_file" "test" {
  path               = %[1]q
  content            = %[2]q
  mode               = %[3]q
  create_directories = true
%[4]s
}
`, path, content, mode, testSSHBlock())
}

func testAccFileConfigWithPublicKey(path, content, mode string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_file" "test" {
  path    = %[1]q
  content = %[2]q
  mode    = %[3]q
%[4]s
}
`, path, content, mode, testSSHBlockWithPublicKey())
}

func TestAccFileResource_overwrite(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFileConfigOverwrite("/tmp/tf_acc_file_overwrite", false),
				ExpectError: regexp.MustCompile("Resource already exists"),
			},
			{
				Config: testAccFileConfigOverwrite("/tmp/tf_acc_file_overwrite", true),
			},
		},
	})
}

func testAccFileConfigOverwrite(path string, overwrite bool) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_file" "setup" {
  path    = %[1]q
  content = "setup"
%[3]s
}

resource "debian_file" "test" {
  path       = %[1]q
  content    = "setup"
  overwrite  = %[2]t
  depends_on = [debian_file.setup]
%[3]s
}
`, path, overwrite, testSSHBlock())
}
