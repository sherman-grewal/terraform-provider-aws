package aws

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/appconfig"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccAWSAppConfigConfigurationProfile_basic(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_appconfig_configuration_profile.test"
	appResourceName := "aws_appconfig_application.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, appconfig.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAppConfigConfigurationProfileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigName(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					testAccMatchResourceAttrRegionalARN(resourceName, "arn", "appconfig", regexp.MustCompile(`application/[a-z0-9]{4,7}/configurationprofile/[a-z0-9]{4,7}`)),
					resource.TestCheckResourceAttrPair(resourceName, "application_id", appResourceName, "id"),
					resource.TestMatchResourceAttr(resourceName, "configuration_profile_id", regexp.MustCompile(`[a-z0-9]{4,7}`)),
					resource.TestCheckResourceAttr(resourceName, "location_uri", "hosted"),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "validator.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSAppConfigConfigurationProfile_disappears(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_appconfig_configuration_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, appconfig.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAppConfigConfigurationProfileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigName(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					testAccCheckResourceDisappears(testAccProvider, resourceAwsAppconfigConfigurationProfile(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAWSAppConfigConfigurationProfile_Validators_JSON(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_appconfig_configuration_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, appconfig.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAppConfigConfigurationProfileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigValidator_JSON(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "validator.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "validator.*", map[string]string{
						"type": appconfig.ValidatorTypeJsonSchema,
					}),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigValidator_NoJSONContent(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "validator.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "validator.*", map[string]string{
						"content": "",
						"type":    appconfig.ValidatorTypeJsonSchema,
					}),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Test Validator Removal
				Config: testAccAWSAppConfigConfigurationProfileConfigName(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "validator.#", "0"),
				),
			},
		},
	})
}

func TestAccAWSAppConfigConfigurationProfile_Validators_Lambda(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_appconfig_configuration_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, appconfig.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAppConfigConfigurationProfileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigValidator_Lambda(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "validator.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(resourceName, "validator.*.content", "aws_lambda_function.test", "arn"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "validator.*", map[string]string{
						"type": appconfig.ValidatorTypeLambda,
					}),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Test Validator Removal
				Config: testAccAWSAppConfigConfigurationProfileConfigName(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "validator.#", "0"),
				),
			},
		},
	})
}

func TestAccAWSAppConfigConfigurationProfile_Validators_Multiple(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_appconfig_configuration_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, appconfig.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAppConfigConfigurationProfileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigValidator_Multiple(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "validator.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "validator.*", map[string]string{
						"content": "{\"$schema\":\"http://json-schema.org/draft-05/schema#\",\"description\":\"BasicFeatureToggle-1\",\"title\":\"$id$\"}",
						"type":    appconfig.ValidatorTypeJsonSchema,
					}),
					resource.TestCheckTypeSetElemAttrPair(resourceName, "validator.*.content", "aws_lambda_function.test", "arn"),
					resource.TestCheckTypeSetElemNestedAttrs(resourceName, "validator.*", map[string]string{
						"type": appconfig.ValidatorTypeLambda,
					}),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSAppConfigConfigurationProfile_updateName(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	rNameUpdated := acctest.RandomWithPrefix("tf-acc-test-update")
	resourceName := "aws_appconfig_configuration_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, appconfig.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAppConfigConfigurationProfileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigName(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
				),
			},
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigName(rNameUpdated),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", rNameUpdated),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSAppConfigConfigurationProfile_updateDescription(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	description := acctest.RandomWithPrefix("tf-acc-test-update")
	resourceName := "aws_appconfig_configuration_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, appconfig.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAppConfigConfigurationProfileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigDescription(rName, rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "description", rName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSAppConfigConfigurationProfileConfigDescription(rName, description),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "description", description),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSAppConfigConfigurationProfile_Tags(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_appconfig_configuration_profile.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, appconfig.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAppConfigConfigurationProfileDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSAppConfigConfigurationProfileTags1(rName, "key1", "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.key1", "value1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSAppConfigConfigurationProfileTags2(rName, "key1", "value1updated", "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.key1", "value1updated"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
			{
				Config: testAccAWSAppConfigConfigurationProfileTags1(rName, "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAppConfigConfigurationProfileExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
		},
	})
}

func testAccCheckAppConfigConfigurationProfileDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*AWSClient).appconfigconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_appconfig_configuration_profile" {
			continue
		}

		confProfID, appID, err := resourceAwsAppconfigConfigurationProfileParseID(rs.Primary.ID)

		if err != nil {
			return err
		}

		input := &appconfig.GetConfigurationProfileInput{
			ApplicationId:          aws.String(appID),
			ConfigurationProfileId: aws.String(confProfID),
		}

		output, err := conn.GetConfigurationProfile(input)

		if tfawserr.ErrCodeEquals(err, appconfig.ErrCodeResourceNotFoundException) {
			continue
		}

		if err != nil {
			return fmt.Errorf("error reading AppConfig Configuration Profile (%s) for Application (%s): %w", confProfID, appID, err)
		}

		if output != nil {
			return fmt.Errorf("AppConfig Configuration Profile (%s) for Application (%s) still exists", confProfID, appID)
		}
	}

	return nil
}

func testAccCheckAWSAppConfigConfigurationProfileExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Resource not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Resource (%s) ID not set", resourceName)
		}

		confProfID, appID, err := resourceAwsAppconfigConfigurationProfileParseID(rs.Primary.ID)

		if err != nil {
			return err
		}

		conn := testAccProvider.Meta().(*AWSClient).appconfigconn

		output, err := conn.GetConfigurationProfile(&appconfig.GetConfigurationProfileInput{
			ApplicationId:          aws.String(appID),
			ConfigurationProfileId: aws.String(confProfID),
		})

		if err != nil {
			return fmt.Errorf("error reading AppConfig Configuration Profile (%s) for Application (%s): %w", confProfID, appID, err)
		}

		if output == nil {
			return fmt.Errorf("AppConfig Configuration Profile (%s) for Application (%s) not found", confProfID, appID)
		}

		return nil
	}
}

func testAccAWSAppConfigConfigurationProfileConfigName(rName string) string {
	return composeConfig(
		testAccAWSAppConfigApplicationConfigName(rName),
		fmt.Sprintf(`
resource "aws_appconfig_configuration_profile" "test" {
  application_id = aws_appconfig_application.test.id
  name           = %q
  location_uri   = "hosted"
}
`, rName))
}

func testAccAWSAppConfigConfigurationProfileConfigDescription(rName, description string) string {
	return composeConfig(
		testAccAWSAppConfigApplicationConfigDescription(rName, description),
		fmt.Sprintf(`
resource "aws_appconfig_configuration_profile" "test" {
  application_id = aws_appconfig_application.test.id
  name           = %[1]q
  description    = %[2]q
  location_uri   = "hosted"
}
`, rName, description))
}

func testAccAWSAppConfigConfigurationProfileConfigValidator_JSON(rName string) string {
	return composeConfig(
		testAccAWSAppConfigApplicationConfigName(rName),
		fmt.Sprintf(`
resource "aws_appconfig_configuration_profile" "test" {
  application_id = aws_appconfig_application.test.id
  name           = %q
  location_uri   = "hosted"

  validator {
    content = jsonencode({
      "$schema"            = "http://json-schema.org/draft-04/schema#"
      title                = "$id$"
      description          = "BasicFeatureToggle-1"
      type                 = "object"
      additionalProperties = false
      patternProperties = {
        "[^\\s]+$" = {
          type = "boolean"
        }
      }
      minProperties = 1
    })

    type = "JSON_SCHEMA"
  }
}
`, rName))
}

func testAccAWSAppConfigConfigurationProfileConfigValidator_NoJSONContent(rName string) string {
	return composeConfig(
		testAccAWSAppConfigApplicationConfigName(rName),
		fmt.Sprintf(`
resource "aws_appconfig_configuration_profile" "test" {
  application_id = aws_appconfig_application.test.id
  name           = %q
  location_uri   = "hosted"

  validator {
    type = "JSON_SCHEMA"
  }
}
`, rName))
}

func testAccAWSAppConfigApplicationConfigLambdaBase(rName string) string {
	return fmt.Sprintf(`
data "aws_partition" "current" {}

resource "aws_iam_role" "lambda" {
  name = "%[1]s-lambda"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.${data.aws_partition.current.dns_suffix}"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_lambda_function" "test" {
  filename      = "test-fixtures/lambdatest.zip"
  function_name = %[1]q
  role          = aws_iam_role.lambda.arn
  handler       = "exports.example"
  runtime       = "nodejs12.x"
}
`, rName)
}

func testAccAWSAppConfigConfigurationProfileConfigValidator_Lambda(rName string) string {
	return composeConfig(
		testAccAWSAppConfigApplicationConfigName(rName),
		testAccAWSAppConfigApplicationConfigLambdaBase(rName),
		fmt.Sprintf(`
resource "aws_appconfig_configuration_profile" "test" {
  application_id = aws_appconfig_application.test.id
  name           = %[1]q
  location_uri   = "hosted"

  validator {
    content = aws_lambda_function.test.arn
    type    = "LAMBDA"
  }
}
`, rName))
}

func testAccAWSAppConfigConfigurationProfileConfigValidator_Multiple(rName string) string {
	return composeConfig(
		testAccAWSAppConfigApplicationConfigName(rName),
		testAccAWSAppConfigApplicationConfigLambdaBase(rName),
		fmt.Sprintf(`
resource "aws_appconfig_configuration_profile" "test" {
  application_id = aws_appconfig_application.test.id
  name           = %[1]q
  location_uri   = "hosted"

  validator {
    content = jsonencode({
      "$schema"   = "http://json-schema.org/draft-05/schema#"
      title       = "$id$"
      description = "BasicFeatureToggle-1"
    })

    type = "JSON_SCHEMA"
  }

  validator {
    content = aws_lambda_function.test.arn
    type    = "LAMBDA"
  }
}
`, rName))
}

func testAccAWSAppConfigConfigurationProfileTags1(rName, tagKey1, tagValue1 string) string {
	return composeConfig(
		testAccAWSAppConfigApplicationConfigName(rName),
		fmt.Sprintf(`
resource "aws_appconfig_configuration_profile" "test" {
  application_id = aws_appconfig_application.test.id
  location_uri   = "hosted"
  name           = %[1]q

  tags = {
    %[2]q = %[3]q
  }
}
`, rName, tagKey1, tagValue1))
}

func testAccAWSAppConfigConfigurationProfileTags2(rName, tagKey1, tagValue1, tagKey2, tagValue2 string) string {
	return composeConfig(
		testAccAWSAppConfigApplicationConfigName(rName),
		fmt.Sprintf(`
resource "aws_appconfig_configuration_profile" "test" {
  application_id = aws_appconfig_application.test.id
  location_uri   = "hosted"
  name           = %[1]q

  tags = {
    %[2]q = %[3]q
    %[4]q = %[5]q
  }
}
`, rName, tagKey1, tagValue1, tagKey2, tagValue2))
}
