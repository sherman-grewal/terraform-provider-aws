package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/lakeformation"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func testAccAWSLakeFormationDataLakeSettingsDataSource_basic(t *testing.T) {
	resourceName := "data.aws_lakeformation_data_lake_settings.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t); testAccPartitionHasServicePreCheck(lakeformation.EndpointsID, t) },
		ErrorCheck:   testAccErrorCheck(t, lakeformation.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSLakeFormationDataLakeSettingsDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSLakeFormationDataLakeSettingsDataSourceConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(resourceName, "catalog_id", "data.aws_caller_identity.current", "account_id"),
					resource.TestCheckResourceAttr(resourceName, "admins.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "admins.0", "data.aws_iam_session_context.current", "issuer_arn"),
				),
			},
		},
	})
}

const testAccAWSLakeFormationDataLakeSettingsDataSourceConfig_basic = `
data "aws_caller_identity" "current" {}

data "aws_iam_session_context" "current" {
  arn = data.aws_caller_identity.current.arn
}

resource "aws_lakeformation_data_lake_settings" "test" {
  catalog_id = data.aws_caller_identity.current.account_id
  admins     = [data.aws_iam_session_context.current.issuer_arn]
}

data "aws_lakeformation_data_lake_settings" "test" {
  catalog_id = aws_lakeformation_data_lake_settings.test.catalog_id
}
`
