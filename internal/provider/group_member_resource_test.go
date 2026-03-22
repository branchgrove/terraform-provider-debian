package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccGroupMemberResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupMemberResourceConfig("tfmemuser1", "tfmemgrp1"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("debian_group_member.test",
						tfjsonpath.New("user"), knownvalue.StringExact("tfmemuser1")),
					statecheck.ExpectKnownValue("debian_group_member.test",
						tfjsonpath.New("group"), knownvalue.StringExact("tfmemgrp1")),
				},
			},
		},
	})
}

func TestAccGroupMemberResource_importState(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupMemberResourceConfig("tfmemuser2", "tfmemgrp2"),
			},
			{
				ResourceName:                         "debian_group_member.test",
				ImportState:                          true,
				ImportStateId:                        testImportID("tfmemuser2:tfmemgrp2"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "user",
			},
		},
	})
}

// --- config helpers ---

func testAccGroupMemberResourceConfig(userName, groupName string) string {
	return testProviderBlock() + fmt.Sprintf(`
resource "debian_user" "test" {
  name = %[1]q
%[3]s
}

resource "debian_group" "test" {
  name = %[2]q
%[3]s
}

resource "debian_group_member" "test" {
  user  = debian_user.test.name
  group = debian_group.test.name
%[3]s
}
`, userName, groupName, testSSHBlock())
}
