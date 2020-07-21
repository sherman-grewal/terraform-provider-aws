package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/guardduty"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func testAccAwsGuardDutyFilter_basic(t *testing.T) {
	resourceName := "aws_guardduty_filter.test"
	detectorResourceName := "aws_guardduty_detector.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAwsGuardDutyFilterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGuardDutyFilterConfig_full(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAwsGuardDutyFilterExists(resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "detector_id", detectorResourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", "test-filter"),
					resource.TestCheckResourceAttr(resourceName, "action", "ARCHIVE"),
					resource.TestCheckResourceAttr(resourceName, "rank", "1"),
					resource.TestCheckResourceAttrSet(resourceName, "finding_criteria"),
				),
			},
			{
				Config: testAccGuardDutyFilterConfigNoop_full(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAwsGuardDutyFilterExists(resourceName),
					resource.TestCheckResourceAttrPair(resourceName, "detector_id", detectorResourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", "test-filter"),
					resource.TestCheckResourceAttr(resourceName, "action", "NOOP"),
					resource.TestCheckResourceAttr(resourceName, "rank", "1"),
					resource.TestCheckResourceAttrSet(resourceName, "finding_criteria"),
				),
			},
		},
	})
}

func testAccAwsGuardDutyFilter_import(t *testing.T) {
	resourceName := "aws_guardduty_filter.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAwsGuardDutyFilterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGuardDutyFilterConfig_full(),
			},

			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckAwsGuardDutyFilterDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*AWSClient).guarddutyconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_guardduty_filter" {
			continue
		}

		detectorID, filterName, err := parseImportedId(rs.Primary.ID)
		if err != nil {
			return err
		}

		input := &guardduty.GetFilterInput{
			DetectorId: aws.String(detectorID),
			FilterName: aws.String(filterName),
		}

		_, err = conn.GetFilter(input)
		if err != nil {
			if isAWSErr(err, guardduty.ErrCodeBadRequestException, "The request is rejected because the input detectorId is not owned by the current account.") {
				return nil
			}
			return err
		}

		return fmt.Errorf("Expected GuardDuty Filter to be destroyed, %s found", rs.Primary.Attributes["filter_name"])
	}

	return nil
}

func testAccCheckAwsGuardDutyFilterExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("Not found: %s", name)
		}

		return nil
	}
}

func testAccGuardDutyFilterConfig_full() string {
	return `
resource "aws_guardduty_filter" "test" {
  detector_id = "${aws_guardduty_detector.test.id}"
	name        = "test-filter"
	action      = "ARCHIVE"
	rank        = 1

  finding_criteria {
    criterion {
      field     = "region"
      values    = ["eu-west-1"]
      condition = "equals"
    }

    criterion {
      field     = "service.additionalInfo.threatListName"
      values    = ["some-threat", "another-threat"]
      condition = "not_equals"
    }

    criterion {
      field     = "updatedAt"
      values    = ["1570744740000"]
      condition = "less_than"
    }

    criterion {
      field     = "updatedAt"
      values    = ["1570744240000"]
      condition = "greater_than"
    }
  }
}

resource "aws_guardduty_detector" "test" {
  enable = true
}`
}

func testAccGuardDutyFilterConfigNoop_full() string {
	return `
resource "aws_guardduty_filter" "test" {
  detector_id = "${aws_guardduty_detector.test.id}"
	name        = "test-filter"
	action      = "NOOP"
	rank        = 1

  finding_criteria {
    criterion {
      field     = "region"
      values    = ["eu-west-1"]
      condition = "equals"
    }

    criterion {
      field     = "service.additionalInfo.threatListName"
      values    = ["some-threat", "another-threat"]
      condition = "not_equals"
    }

    criterion {
      field     = "updatedAt"
      values    = ["1570744740000"]
      condition = "less_than"
    }

    criterion {
      field     = "updatedAt"
      values    = ["1570744240000"]
      condition = "greater_than"
    }
  }
}

resource "aws_guardduty_detector" "test" {
  enable = true
}`
}
