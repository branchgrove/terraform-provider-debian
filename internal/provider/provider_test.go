package provider

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"debian": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, env := range []string{"TEST_SSH_HOST", "TEST_SSH_PRIVATE_KEY", "TEST_SSH_PUBLIC_KEY"} {
		if os.Getenv(env) == "" {
			t.Fatalf("%s must be set for acceptance tests", env)
		}
	}
}

func testProviderBlock() string {
	return `
provider "debian" {
  private_key = <<-EOT
` + os.Getenv("TEST_SSH_PRIVATE_KEY") + `
  EOT
  private_keys = {
    "` + os.Getenv("TEST_SSH_PUBLIC_KEY") + `" = <<-EOT
` + os.Getenv("TEST_SSH_PRIVATE_KEY") + `
    EOT
  }
}
`
}

func testSSHBlock() string {
	return `
    ssh = {
      hostname = "` + os.Getenv("TEST_SSH_HOST") + `"
    }
`
}

func testSSHBlockWithPublicKey() string {
	return `
    ssh = {
      hostname   = "` + os.Getenv("TEST_SSH_HOST") + `"
      public_key = "` + os.Getenv("TEST_SSH_PUBLIC_KEY") + `"
    }
`
}

func testImportID(resourceID string) string {
	return "user=root;host=" + os.Getenv("TEST_SSH_HOST") + ";port=22;id=" + resourceID
}

func testImportIDWithKey(resourceID string) string {
	pub := os.Getenv("TEST_SSH_PUBLIC_KEY")
	parts := strings.Fields(pub)
	if len(parts) >= 2 {
		pub = parts[0] + " " + parts[1]
	}
	return "user=root;host=" + os.Getenv("TEST_SSH_HOST") + ";port=22;public_key=" + pub + ";id=" + resourceID
}
