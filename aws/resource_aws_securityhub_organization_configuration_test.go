package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/securityhub"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func testAccAwsSecurityHubOrganizationConfiguration_basic(t *testing.T) {
	resourceName := "aws_securityhub_organization_configuration.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t); testAccOrganizationsAccountPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, securityhub.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: nil, //lintignore:AT001
		Steps: []resource.TestStep{
			{
				Config: testAccAwsSecurityHubOrganizationConfigurationConfig(true),
				Check: resource.ComposeTestCheckFunc(
					testAccAwsSecurityHubOrganizationConfigurationExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "auto_enable", "true"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAwsSecurityHubOrganizationConfigurationConfig(false),
				Check: resource.ComposeTestCheckFunc(
					testAccAwsSecurityHubOrganizationConfigurationExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "auto_enable", "false"),
				),
			},
		},
	})
}

func testAccAwsSecurityHubOrganizationConfigurationExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := testAccProvider.Meta().(*AWSClient).securityhubconn

		_, err := conn.DescribeOrganizationConfiguration(&securityhub.DescribeOrganizationConfigurationInput{})

		return err
	}
}

func testAccAwsSecurityHubOrganizationConfigurationConfig(autoEnable bool) string {
	return fmt.Sprintf(`
data "aws_partition" "current" {}

resource "aws_organizations_organization" "test" {
  aws_service_access_principals = ["securityhub.${data.aws_partition.current.dns_suffix}"]
  feature_set                   = "ALL"
}

resource "aws_securityhub_account" "test" {}

data "aws_caller_identity" "current" {}

resource "aws_securityhub_organization_admin_account" "test" {
  admin_account_id = data.aws_caller_identity.current.account_id

  depends_on = [aws_organizations_organization.test, aws_securityhub_account.test]
}

resource "aws_securityhub_organization_configuration" "test" {
  auto_enable = %[1]t

  depends_on = [aws_securityhub_organization_admin_account.test]
}
`, autoEnable)
}
