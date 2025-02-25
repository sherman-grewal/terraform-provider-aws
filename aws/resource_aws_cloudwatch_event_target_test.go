package aws

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	events "github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/naming"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/service/cloudwatchevents/finder"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/service/cloudwatchevents/lister"
)

func init() {
	resource.AddTestSweepers("aws_cloudwatch_event_target", &resource.Sweeper{
		Name: "aws_cloudwatch_event_target",
		F:    testSweepCloudWatchEventTargets,
	})
}

func testSweepCloudWatchEventTargets(region string) error {
	client, err := sharedClientForRegion(region)
	if err != nil {
		return fmt.Errorf("Error getting client: %w", err)
	}
	conn := client.(*AWSClient).cloudwatcheventsconn

	var sweeperErrs *multierror.Error
	var rulesCount, targetsCount int

	rulesInput := &events.ListRulesInput{}

	err = lister.ListRulesPages(conn, rulesInput, func(rulesPage *events.ListRulesOutput, lastPage bool) bool {
		if rulesPage == nil {
			return !lastPage
		}

		for _, rule := range rulesPage.Rules {
			rulesCount++
			ruleName := aws.StringValue(rule.Name)

			log.Printf("[INFO] Deleting CloudWatch Events targets for rule (%s)", ruleName)
			targetsInput := &events.ListTargetsByRuleInput{
				Rule:  rule.Name,
				Limit: aws.Int64(100), // Set limit to allowed maximum to prevent API throttling
			}

			err := lister.ListTargetsByRulePages(conn, targetsInput, func(targetsPage *events.ListTargetsByRuleOutput, lastPage bool) bool {
				if targetsPage == nil {
					return !lastPage
				}

				for _, target := range targetsPage.Targets {
					targetsCount++
					removeTargetsInput := &events.RemoveTargetsInput{
						Ids:   []*string{target.Id},
						Rule:  rule.Name,
						Force: aws.Bool(true), // Required for AWS-managed rules, ignored otherwise
					}
					targetID := aws.StringValue(target.Id)

					log.Printf("[INFO] Deleting CloudWatch Events target (%s/%s)", ruleName, targetID)
					_, err := conn.RemoveTargets(removeTargetsInput)

					if err != nil {
						sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("Error deleting CloudWatch Events target (%s/%s): %w", ruleName, targetID, err))
						continue
					}
				}

				return !lastPage
			})

			if testSweepSkipSweepError(err) {
				log.Printf("[WARN] Skipping CloudWatch Events target sweeper for %q: %s", region, err)
				return false
			}
			if err != nil {
				sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error listing CloudWatch Events targets for rule (%s): %w", ruleName, err))
			}
		}

		return !lastPage
	})

	if testSweepSkipSweepError(err) {
		log.Printf("[WARN] Skipping CloudWatch Events rule target sweeper for %q: %s", region, err)
		return sweeperErrs.ErrorOrNil() // In case we have completed some pages, but had errors
	}

	if err != nil {
		sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error listing CloudWatch Events rules: %w", err))
	}

	log.Printf("[INFO] Deleted %d CloudWatch Events targets across %d CloudWatch Events rules", targetsCount, rulesCount)

	return sweeperErrs.ErrorOrNil()
}

func TestAccAWSCloudWatchEventTarget_basic(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	snsTopicResourceName := "aws_sns_topic.test"

	var v1, v2 events.Target
	ruleName := acctest.RandomWithPrefix("tf-acc-test-rule")
	snsTopicName1 := acctest.RandomWithPrefix("tf-acc-test-sns")
	snsTopicName2 := acctest.RandomWithPrefix("tf-acc-test-sns")
	targetID1 := acctest.RandomWithPrefix("tf-acc-test-target")
	targetID2 := acctest.RandomWithPrefix("tf-acc-test-target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfig(ruleName, snsTopicName1, targetID1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v1),
					resource.TestCheckResourceAttr(resourceName, "rule", ruleName),
					resource.TestCheckResourceAttr(resourceName, "event_bus_name", "default"),
					resource.TestCheckResourceAttr(resourceName, "target_id", targetID1),
					resource.TestCheckResourceAttrPair(resourceName, "arn", snsTopicResourceName, "arn"),

					resource.TestCheckResourceAttr(resourceName, "input", ""),
					resource.TestCheckResourceAttr(resourceName, "input_path", ""),
					resource.TestCheckResourceAttr(resourceName, "role_arn", ""),
					resource.TestCheckResourceAttr(resourceName, "run_command_targets.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "batch_target.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "http_target.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "kinesis_target.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "sqs_target.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "input_transformer.#", "0"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetNoBusNameImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSCloudWatchEventTargetConfig(ruleName, snsTopicName2, targetID2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v2),
					resource.TestCheckResourceAttr(resourceName, "rule", ruleName),
					resource.TestCheckResourceAttr(resourceName, "event_bus_name", "default"),
					resource.TestCheckResourceAttr(resourceName, "target_id", targetID2),
					resource.TestCheckResourceAttrPair(resourceName, "arn", snsTopicResourceName, "arn"),
				),
			},
			{
				Config:   testAccAWSCloudWatchEventTargetConfigDefaultEventBusName(ruleName, snsTopicName2, targetID2),
				PlanOnly: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_EventBusName(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"

	var v1, v2 events.Target
	ruleName := acctest.RandomWithPrefix("tf-acc-test-rule")
	busName := acctest.RandomWithPrefix("tf-acc-test-bus")
	snsTopicName1 := acctest.RandomWithPrefix("tf-acc-test-sns")
	snsTopicName2 := acctest.RandomWithPrefix("tf-acc-test-sns")
	targetID1 := acctest.RandomWithPrefix("tf-acc-test-target")
	targetID2 := acctest.RandomWithPrefix("tf-acc-test-target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigEventBusName(ruleName, busName, snsTopicName1, targetID1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v1),
					resource.TestCheckResourceAttr(resourceName, "rule", ruleName),
					resource.TestCheckResourceAttr(resourceName, "event_bus_name", busName),
					resource.TestCheckResourceAttr(resourceName, "target_id", targetID1),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSCloudWatchEventTargetConfigEventBusName(ruleName, busName, snsTopicName2, targetID2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v2),
					resource.TestCheckResourceAttr(resourceName, "rule", ruleName),
					resource.TestCheckResourceAttr(resourceName, "event_bus_name", busName),
					resource.TestCheckResourceAttr(resourceName, "target_id", targetID2),
				),
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_GeneratedTargetId(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	snsTopicResourceName := "aws_sns_topic.test"

	var v events.Target
	ruleName := acctest.RandomWithPrefix("tf-acc-cw-event-rule-missing-target-id")
	snsTopicName := acctest.RandomWithPrefix("tf-acc")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigMissingTargetId(ruleName, snsTopicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "rule", ruleName),
					resource.TestCheckResourceAttrPair(resourceName, "arn", snsTopicResourceName, "arn"),
					naming.TestCheckResourceAttrNameGenerated(resourceName, "target_id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_RetryPolicy_DeadLetterConfig(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	kinesisStreamResourceName := "aws_kinesis_stream.test"
	queueResourceName := "aws_sqs_queue.test"
	var v events.Target

	ruleName := acctest.RandomWithPrefix("tf-acc-cw-event-rule-full")
	ssmDocumentName := acctest.RandomWithPrefix("tf_ssm_Document")
	targetID := acctest.RandomWithPrefix("tf-acc-cw-target-full")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfig_retryPolicyDlc(ruleName, targetID, ssmDocumentName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "rule", ruleName),
					resource.TestCheckResourceAttr(resourceName, "target_id", targetID),
					resource.TestCheckResourceAttrPair(resourceName, "arn", kinesisStreamResourceName, "arn"),
					testAccCheckResourceAttrEquivalentJSON(resourceName, "input", `{"source": ["aws.cloudtrail"]}`),
					resource.TestCheckResourceAttr(resourceName, "input_path", ""),
					resource.TestCheckResourceAttr(resourceName, "retry_policy.0.maximum_event_age_in_seconds", "60"),
					resource.TestCheckResourceAttr(resourceName, "retry_policy.0.maximum_retry_attempts", "5"),
					resource.TestCheckResourceAttrPair(resourceName, "dead_letter_config.0.arn", queueResourceName, "arn"),
				),
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_full(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	kinesisStreamResourceName := "aws_kinesis_stream.test"
	var v events.Target

	ruleName := acctest.RandomWithPrefix("tf-acc-cw-event-rule-full")
	ssmDocumentName := acctest.RandomWithPrefix("tf_ssm_Document")
	targetID := acctest.RandomWithPrefix("tf-acc-cw-target-full")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfig_full(ruleName, targetID, ssmDocumentName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "rule", ruleName),
					resource.TestCheckResourceAttr(resourceName, "target_id", targetID),
					resource.TestCheckResourceAttrPair(resourceName, "arn", kinesisStreamResourceName, "arn"),
					testAccCheckResourceAttrEquivalentJSON(resourceName, "input", `{"source": ["aws.cloudtrail"]}`),
					resource.TestCheckResourceAttr(resourceName, "input_path", ""),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_disappears(t *testing.T) {
	var v events.Target

	ruleName := acctest.RandomWithPrefix("tf-acc-test")
	snsTopicName := acctest.RandomWithPrefix("tf-acc-test-sns")
	targetID := acctest.RandomWithPrefix("tf-acc-test-target")

	resourceName := "aws_cloudwatch_event_target.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfig(ruleName, snsTopicName, targetID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					testAccCheckResourceDisappears(testAccProvider, resourceAwsCloudWatchEventTarget(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_ssmDocument(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_ssm_Document")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigSsmDocument(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "run_command_targets.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "run_command_targets.0.key", "tag:Name"),
					resource.TestCheckResourceAttr(resourceName, "run_command_targets.0.values.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "run_command_targets.0.values.0", "acceptance_test"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_http(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"

	var v events.Target
	rName := acctest.RandomWithPrefix("tf_http_target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigHttp(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "http_target.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "http_target.0.path_parameter_values.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "http_target.0.header_parameters.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "http_target.0.header_parameters.X-Test", "test"),
					resource.TestCheckResourceAttr(resourceName, "http_target.0.query_string_parameters.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "http_target.0.query_string_parameters.Env", "test"),
					resource.TestCheckResourceAttr(resourceName, "http_target.0.query_string_parameters.Path", "$.detail.path"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_ecs(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	iamRoleResourceName := "aws_iam_role.test"
	ecsTaskDefinitionResourceName := "aws_ecs_task_definition.task"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_ecs_target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigEcs(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttrPair(resourceName, "role_arn", iamRoleResourceName, "arn"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.task_count", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "ecs_target.0.task_definition_arn", ecsTaskDefinitionResourceName, "arn"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.launch_type", "FARGATE"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.network_configuration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.network_configuration.0.subnets.#", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_redshift(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	iamRoleResourceName := "aws_iam_role.test"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_ecs_target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigRedshift(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttrPair(resourceName, "role_arn", iamRoleResourceName, "arn"),
					resource.TestCheckResourceAttr(resourceName, "redshift_target.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "redshift_target.0.database", "redshiftdb"),
					resource.TestCheckResourceAttr(resourceName, "redshift_target.0.sql", "SELECT * FROM table"),
					resource.TestCheckResourceAttr(resourceName, "redshift_target.0.statement_name", "NewStatement"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_ecsWithBlankLaunchType(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	iamRoleResourceName := "aws_iam_role.test"
	ecsTaskDefinitionResourceName := "aws_ecs_task_definition.task"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_ecs_target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigEcsWithBlankLaunchType(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttrPair(resourceName, "role_arn", iamRoleResourceName, "arn"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.task_count", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "ecs_target.0.task_definition_arn", ecsTaskDefinitionResourceName, "arn"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.launch_type", ""),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.network_configuration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.network_configuration.0.subnets.#", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSCloudWatchEventTargetConfigEcs(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.launch_type", "FARGATE"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSCloudWatchEventTargetConfigEcsWithBlankLaunchType(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.launch_type", ""),
				),
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_ecsWithBlankTaskCount(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_ecs_target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigEcsWithBlankTaskCount(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.task_count", "1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_ecsFull(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_ecs_target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigEcsWithBlankTaskCountFull(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.task_count", "1"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.launch_type", "FARGATE"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.enable_execute_command", "true"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.enable_ecs_managed_tags", "true"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.propagate_tags", "TASK_DEFINITION"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.placement_constraint.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.placement_constraint.0.type", "distinctInstance"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "ecs_target.0.tags.test", "test1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_batch(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	batchJobDefinitionResourceName := "aws_batch_job_definition.test"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_batch_target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigBatch(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "batch_target.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "batch_target.0.job_definition", batchJobDefinitionResourceName, "arn"),
					resource.TestCheckResourceAttr(resourceName, "batch_target.0.job_name", rName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_kinesis(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_kinesis_target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigKinesis(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "kinesis_target.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "kinesis_target.0.partition_key_path", "$.detail"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName), ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_sqs(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_sqs_target")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigSqs(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "sqs_target.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "sqs_target.0.message_group_id", "event_group"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_input_transformer(t *testing.T) {
	resourceName := "aws_cloudwatch_event_target.test"
	var v events.Target
	rName := acctest.RandomWithPrefix("tf_input_transformer")

	tooManyInputPaths := make([]string, 101)
	for i := range tooManyInputPaths {
		tooManyInputPaths[i] = fmt.Sprintf("InvalidField_%d", i)
	}

	validInputPaths := make([]string, 100)
	for i := range validInputPaths {
		validInputPaths[i] = fmt.Sprintf("ValidField_%d", i)
	}

	var expectedInputTemplate strings.Builder
	fmt.Fprintf(&expectedInputTemplate, `{
  "detail-type": "Scheduled Event",
  "source": "aws.events",
`)
	for _, path := range validInputPaths {
		fmt.Fprintf(&expectedInputTemplate, "  \"%[1]s\": <%[1]s>,\n", path)
	}
	fmt.Fprintf(&expectedInputTemplate, `  "detail": {}
}
`)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccAWSCloudWatchEventTargetConfigInputTransformer(rName, tooManyInputPaths),
				ExpectError: regexp.MustCompile(`.*expected number of items in.* to be less than or equal to.*`),
			},
			{
				Config: testAccAWSCloudWatchEventTargetConfigInputTransformer(rName, validInputPaths),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &v),
					resource.TestCheckResourceAttr(resourceName, "input_transformer.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "input_transformer.0.input_paths.%", strconv.Itoa(len(validInputPaths))),
					resource.TestCheckResourceAttr(resourceName, "input_transformer.0.input_paths.ValidField_99", "$.ValidField_99"),
					resource.TestCheckResourceAttr(resourceName, "input_transformer.0.input_template", expectedInputTemplate.String()),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_inputTransformerJsonString(t *testing.T) {
	var target events.Target
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resourceName := "aws_cloudwatch_event_target.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetConfigInputTransformerJsonString(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &target),
					resource.TestCheckResourceAttr(resourceName, "input_transformer.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "input_transformer.0.input_paths.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "input_transformer.0.input_paths.instance", "$.detail.instance"),
					resource.TestCheckResourceAttr(resourceName, "input_transformer.0.input_template", "\"<instance> is in state <status>\""),
				),
			},
		},
	})
}

func TestAccAWSCloudWatchEventTarget_PartnerEventBus(t *testing.T) {
	key := "EVENT_BRIDGE_PARTNER_EVENT_BUS_NAME"
	busName := os.Getenv(key)
	if busName == "" {
		t.Skipf("Environment variable %s is not set", key)
	}

	var target events.Target
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_cloudwatch_event_target.test"
	snsTopicResourceName := "aws_sns_topic.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, events.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchEventTargetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSCloudWatchEventTargetPartnerEventBusConfig(rName, busName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckCloudWatchEventTargetExists(resourceName, &target),
					resource.TestCheckResourceAttr(resourceName, "rule", rName),
					resource.TestCheckResourceAttr(resourceName, "event_bus_name", busName),
					resource.TestCheckResourceAttr(resourceName, "target_id", rName),
					resource.TestCheckResourceAttrPair(resourceName, "arn", snsTopicResourceName, "arn"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckCloudWatchEventTargetExists(n string, rule *events.Target) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := testAccProvider.Meta().(*AWSClient).cloudwatcheventsconn
		t, err := finder.Target(conn, rs.Primary.Attributes["event_bus_name"], rs.Primary.Attributes["rule"], rs.Primary.Attributes["target_id"])
		if err != nil {
			return fmt.Errorf("Event Target not found: %w", err)
		}

		*rule = *t

		return nil
	}
}

func testAccCheckAWSCloudWatchEventTargetDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*AWSClient).cloudwatcheventsconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_cloudwatch_event_target" {
			continue
		}

		t, err := finder.Target(conn, rs.Primary.Attributes["event_bus_name"], rs.Primary.Attributes["rule"], rs.Primary.Attributes["target_id"])
		if err == nil {
			return fmt.Errorf("CloudWatch Events Target %q still exists: %s",
				rs.Primary.ID, t)
		}
	}

	return nil
}

func testAccAWSCloudWatchEventTargetImportStateIdFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", resourceName)
		}

		return fmt.Sprintf("%s/%s/%s", rs.Primary.Attributes["event_bus_name"], rs.Primary.Attributes["rule"], rs.Primary.Attributes["target_id"]), nil
	}
}

func testAccAWSCloudWatchEventTargetNoBusNameImportStateIdFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("Not found: %s", resourceName)
		}

		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["rule"], rs.Primary.Attributes["target_id"]), nil
	}
}

func testAccAWSCloudWatchEventTargetConfig(ruleName, snsTopicName, targetID string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name                = "%s"
  schedule_expression = "rate(1 hour)"
}

resource "aws_cloudwatch_event_target" "test" {
  rule      = aws_cloudwatch_event_rule.test.name
  target_id = "%s"
  arn       = aws_sns_topic.test.arn
}

resource "aws_sns_topic" "test" {
  name = "%s"
}
`, ruleName, targetID, snsTopicName)
}

func testAccAWSCloudWatchEventTargetConfigDefaultEventBusName(ruleName, snsTopicName, targetID string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name                = "%s"
  event_bus_name      = "default"
  schedule_expression = "rate(1 hour)"
}

resource "aws_cloudwatch_event_target" "test" {
  rule           = aws_cloudwatch_event_rule.test.name
  event_bus_name = aws_cloudwatch_event_rule.test.event_bus_name
  target_id      = "%s"
  arn            = aws_sns_topic.test.arn
}

resource "aws_sns_topic" "test" {
  name = "%s"
}
`, ruleName, targetID, snsTopicName)
}

func testAccAWSCloudWatchEventTargetConfigEventBusName(ruleName, eventBusName, snsTopicName, targetID string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_target" "test" {
  rule           = aws_cloudwatch_event_rule.test.name
  event_bus_name = aws_cloudwatch_event_rule.test.event_bus_name
  target_id      = %[1]q
  arn            = aws_sns_topic.test.arn
}

resource "aws_sns_topic" "test" {
  name = %[2]q
}

resource "aws_cloudwatch_event_rule" "test" {
  name           = %[3]q
  event_bus_name = aws_cloudwatch_event_bus.test.name
  event_pattern  = <<PATTERN
{
	"source": [
		"aws.ec2"
	]
}
PATTERN
}

resource "aws_cloudwatch_event_bus" "test" {
  name = %[4]q
}
`, targetID, snsTopicName, ruleName, eventBusName)
}

func testAccAWSCloudWatchEventTargetConfigMissingTargetId(ruleName, snsTopicName string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name                = "%s"
  schedule_expression = "rate(1 hour)"
}

resource "aws_cloudwatch_event_target" "test" {
  rule = aws_cloudwatch_event_rule.test.name
  arn  = aws_sns_topic.test.arn
}

resource "aws_sns_topic" "test" {
  name = "%s"
}
`, ruleName, snsTopicName)
}

func testAccAWSCloudWatchEventTargetConfig_retryPolicyDlc(ruleName, targetName, rName string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name                = %[1]q
  schedule_expression = "rate(1 hour)"
  role_arn            = aws_iam_role.test.arn
}

resource "aws_iam_role" "test" {
  name = %[2]q

  assume_role_policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "events.${data.aws_partition.current.dns_suffix}"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
POLICY
}

resource "aws_iam_role_policy" "test" {
  name = "%[2]s_policy"
  role = aws_iam_role.test.id

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "kinesis:PutRecord",
        "kinesis:PutRecords"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "aws_sqs_queue" "test" {
}

resource "aws_cloudwatch_event_target" "test" {
  rule      = aws_cloudwatch_event_rule.test.name
  target_id = %[3]q

  input = <<INPUT
{ "source": ["aws.cloudtrail"] }
INPUT

  arn = aws_kinesis_stream.test.arn

  retry_policy {
    maximum_event_age_in_seconds = 60
    maximum_retry_attempts       = 5
  }

  dead_letter_config {
    arn = aws_sqs_queue.test.arn
  }
}

resource "aws_kinesis_stream" "test" {
  name        = "%[2]s_kinesis_test"
  shard_count = 1
}

data "aws_partition" "current" {}
`, ruleName, rName, targetName)
}

func testAccAWSCloudWatchEventTargetConfig_full(ruleName, targetName, rName string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name                = %[1]q
  schedule_expression = "rate(1 hour)"
  role_arn            = aws_iam_role.test.arn
}

resource "aws_iam_role" "test" {
  name = %[2]q

  assume_role_policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "events.${data.aws_partition.current.dns_suffix}"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
POLICY
}

resource "aws_iam_role_policy" "test" {
  name = "%[2]s_policy"
  role = aws_iam_role.test.id

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "kinesis:PutRecord",
        "kinesis:PutRecords"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "aws_cloudwatch_event_target" "test" {
  rule      = aws_cloudwatch_event_rule.test.name
  target_id = %[3]q

  input = <<INPUT
{ "source": ["aws.cloudtrail"] }
INPUT

  arn = aws_kinesis_stream.test.arn
}

resource "aws_kinesis_stream" "test" {
  name        = "%[2]s_kinesis_test"
  shard_count = 1
}

data "aws_partition" "current" {}
`, ruleName, rName, targetName)
}

func testAccAWSCloudWatchEventTargetConfigSsmDocument(rName string) string {
	return fmt.Sprintf(`
resource "aws_ssm_document" "test" {
  name          = %[1]q
  document_type = "Command"

  content = <<DOC
    {
      "schemaVersion": "1.2",
      "description": "Check ip configuration of a Linux instance.",
      "parameters": {

      },
      "runtimeConfig": {
        "aws:runShellScript": {
          "properties": [
            {
              "id": "0.aws:runShellScript",
              "runCommand": ["ifconfig"]
            }
          ]
        }
      }
    }
DOC
}

resource "aws_cloudwatch_event_rule" "test" {
  name        = %[1]q
  description = "another_test"

  event_pattern = <<PATTERN
{
  "source": [
    "aws.autoscaling"
  ]
}
PATTERN
}

resource "aws_cloudwatch_event_target" "test" {
  arn      = aws_ssm_document.test.arn
  rule     = aws_cloudwatch_event_rule.test.id
  role_arn = aws_iam_role.test.arn

  run_command_targets {
    key    = "tag:Name"
    values = ["acceptance_test"]
  }
}

resource "aws_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "events.${data.aws_partition.current.dns_suffix}"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_role_policy" "test" {
  name = %[1]q
  role = aws_iam_role.test.id

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "ssm:*",
            "Effect": "Allow",
            "Resource": [
                "*"
            ]
        }
    ]
}
EOF
}

data "aws_partition" "current" {}
`, rName)
}

func testAccAWSCloudWatchEventTargetConfigHttp(rName string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name        = %[1]q
  description = "schedule_http_test"

  schedule_expression = "rate(5 minutes)"
}

resource "aws_cloudwatch_event_target" "test" {
  arn  = "${aws_api_gateway_stage.test.execution_arn}/GET"
  rule = aws_cloudwatch_event_rule.test.id

  http_target {
    path_parameter_values = []
    query_string_parameters = {
      Env  = "test"
      Path = "$.detail.path"
    }
    header_parameters = {
      X-Test = "test"
    }
  }
}

resource "aws_api_gateway_rest_api" "test" {
  name = %[1]q
  body = jsonencode({
    openapi = "3.0.1"
    info = {
      title   = "example"
      version = "1.0"
    }
    paths = {
      "/" = {
        get = {
          x-amazon-apigateway-integration = {
            httpMethod           = "GET"
            payloadFormatVersion = "1.0"
            type                 = "HTTP_PROXY"
            uri                  = "https://ip-ranges.amazonaws.com"
          }
        }
      }
    }
  })
}

resource "aws_api_gateway_deployment" "test" {
  rest_api_id = aws_api_gateway_rest_api.test.id

  triggers = {
    redeployment = sha1(jsonencode(aws_api_gateway_rest_api.test.body))
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_api_gateway_stage" "test" {
  deployment_id = aws_api_gateway_deployment.test.id
  rest_api_id   = aws_api_gateway_rest_api.test.id
  stage_name    = "test"
}

data "aws_partition" "current" {}
`, rName)
}

func testAccAWSCloudWatchEventTargetConfigEcsBase(rName string) string {
	return fmt.Sprintf(`
resource "aws_vpc" "vpc" {
  cidr_block = "10.1.0.0/16"
}

resource "aws_subnet" "subnet" {
  vpc_id     = aws_vpc.vpc.id
  cidr_block = "10.1.1.0/24"
}

resource "aws_iam_role" "test" {
  name = %[1]q

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "events.${data.aws_partition.current.dns_suffix}"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_role_policy" "test" {
  name = %[1]q
  role = aws_iam_role.test.id

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ecs:RunTask"
            ],
            "Resource": [
                "*"
            ]
        }
    ]
}
EOF
}

resource "aws_ecs_cluster" "test" {
  name = %[1]q
}

resource "aws_ecs_task_definition" "task" {
  family                   = %[1]q
  cpu                      = 256
  memory                   = 512
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"

  container_definitions = <<EOF
[
  {
    "name": "first",
    "image": "service-first",
    "cpu": 10,
    "memory": 512,
    "essential": true
  }
]
EOF
}

data "aws_partition" "current" {}

resource "aws_cloudwatch_event_rule" "test" {
  name        = %[1]q
  description = "schedule_ecs_test"

  schedule_expression = "rate(5 minutes)"
}
`, rName)
}

func testAccAWSCloudWatchEventTargetConfigEcs(rName string) string {
	return testAccAWSCloudWatchEventTargetConfigEcsBase(rName) + `
resource "aws_cloudwatch_event_target" "test" {
  arn      = aws_ecs_cluster.test.id
  rule     = aws_cloudwatch_event_rule.test.id
  role_arn = aws_iam_role.test.arn

  ecs_target {
    task_count          = 1
    task_definition_arn = aws_ecs_task_definition.task.arn
    launch_type         = "FARGATE"

    network_configuration {
      subnets = [aws_subnet.subnet.id]
    }
  }
}
`
}

func testAccAWSCloudWatchEventTargetConfigRedshift(rName string) string {
	return composeConfig(testAccAWSCloudWatchEventTargetConfigEcsBase(rName),
		testAccAvailableAZsNoOptInConfig(),
		fmt.Sprintf(`
resource "aws_cloudwatch_event_target" "test" {
  arn      = aws_redshift_cluster.default.arn
  rule     = aws_cloudwatch_event_rule.test.id
  role_arn = aws_iam_role.test.arn

  redshift_target {
    database       = "redshiftdb"
    sql            = "SELECT * FROM table"
    statement_name = "NewStatement"
    db_user        = "someUser"
  }
}
resource "aws_redshift_cluster" "default" {
  cluster_identifier                  = "tf-redshift-cluster-%d"
  database_name                       = "mydb"
  master_username                     = "foo_test"
  master_password                     = "Mustbe8characters"
  node_type                           = "dc1.large"
  automated_snapshot_retention_period = 0
  allow_version_upgrade               = false
  skip_final_snapshot                 = true
}
`, 123))
}

func testAccAWSCloudWatchEventTargetConfigEcsWithBlankLaunchType(rName string) string {
	return testAccAWSCloudWatchEventTargetConfigEcsBase(rName) + `
resource "aws_cloudwatch_event_target" "test" {
  arn      = aws_ecs_cluster.test.id
  rule     = aws_cloudwatch_event_rule.test.id
  role_arn = aws_iam_role.test.arn

  ecs_target {
    task_count          = 1
    task_definition_arn = aws_ecs_task_definition.task.arn
    launch_type         = ""

    network_configuration {
      subnets = [aws_subnet.subnet.id]
    }
  }
}
`
}

func testAccAWSCloudWatchEventTargetConfigEcsWithBlankTaskCount(rName string) string {
	return testAccAWSCloudWatchEventTargetConfigEcsBase(rName) + `
resource "aws_cloudwatch_event_target" "test" {
  arn      = aws_ecs_cluster.test.id
  rule     = aws_cloudwatch_event_rule.test.id
  role_arn = aws_iam_role.test.arn

  ecs_target {
    task_definition_arn = aws_ecs_task_definition.task.arn
    launch_type         = "FARGATE"

    network_configuration {
      subnets = [aws_subnet.subnet.id]
    }
  }
}
`
}

func testAccAWSCloudWatchEventTargetConfigEcsWithBlankTaskCountFull(rName string) string {
	return testAccAWSCloudWatchEventTargetConfigEcsBase(rName) + `
resource "aws_cloudwatch_event_target" "test" {
  arn      = aws_ecs_cluster.test.id
  rule     = aws_cloudwatch_event_rule.test.id
  role_arn = aws_iam_role.test.arn

  ecs_target {
    task_definition_arn     = aws_ecs_task_definition.task.arn
    launch_type             = "FARGATE"
    enable_execute_command  = true
    enable_ecs_managed_tags = true
    propagate_tags          = "TASK_DEFINITION"

    placement_constraint {
      type = "distinctInstance"
    }

    tags = {
      test = "test1"
    }

    network_configuration {
      subnets = [aws_subnet.subnet.id]
    }
  }
}
`
}

func testAccAWSCloudWatchEventTargetConfigBatch(rName string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name                = "%[1]s"
  description         = "schedule_batch_test"
  schedule_expression = "rate(5 minutes)"
}

resource "aws_cloudwatch_event_target" "test" {
  arn      = aws_batch_job_queue.test.arn
  rule     = aws_cloudwatch_event_rule.test.id
  role_arn = aws_iam_role.event_iam_role.arn

  batch_target {
    job_definition = aws_batch_job_definition.test.arn
    job_name       = "%[1]s"
  }

  depends_on = [
    "aws_batch_job_queue.test",
    "aws_batch_job_definition.test",
    "aws_iam_role.event_iam_role",
  ]
}

data "aws_partition" "current" {}

resource "aws_iam_role" "event_iam_role" {
  name = "event_%[1]s"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": "events.${data.aws_partition.current.dns_suffix}"
      }
    }
  ]
}
EOF
}

resource "aws_iam_role" "ecs_iam_role" {
  name = "ecs_%[1]s"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.${data.aws_partition.current.dns_suffix}"
      }
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "ecs_policy_attachment" {
  role       = aws_iam_role.ecs_iam_role.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/service-role/AmazonEC2ContainerServiceforEC2Role"
}

resource "aws_iam_instance_profile" "iam_instance_profile" {
  name = "ecs_%[1]s"
  role = aws_iam_role.ecs_iam_role.name
}

resource "aws_iam_role" "batch_iam_role" {
  name = "batch_%[1]s"

  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
    {
        "Action": "sts:AssumeRole",
        "Effect": "Allow",
        "Principal": {
          "Service": "batch.${data.aws_partition.current.dns_suffix}"
        }
    }
    ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "batch_policy_attachment" {
  role       = aws_iam_role.batch_iam_role.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/service-role/AWSBatchServiceRole"
}

resource "aws_security_group" "security_group" {
  name = "%[1]s"
}

resource "aws_vpc" "vpc" {
  cidr_block = "10.1.0.0/16"
}

resource "aws_subnet" "subnet" {
  vpc_id     = aws_vpc.vpc.id
  cidr_block = "10.1.1.0/24"
}

resource "aws_batch_compute_environment" "test" {
  compute_environment_name = "%[1]s"

  compute_resources {
    instance_role = aws_iam_instance_profile.iam_instance_profile.arn

    instance_type = [
      "c4.large",
    ]

    max_vcpus = 16
    min_vcpus = 0

    security_group_ids = [
      aws_security_group.security_group.id,
    ]

    subnets = [
      aws_subnet.subnet.id,
    ]

    type = "EC2"
  }

  service_role = aws_iam_role.batch_iam_role.arn
  type         = "MANAGED"
  depends_on   = [aws_iam_role_policy_attachment.batch_policy_attachment]
}

resource "aws_batch_job_queue" "test" {
  name                 = "%[1]s"
  state                = "ENABLED"
  priority             = 1
  compute_environments = [aws_batch_compute_environment.test.arn]
}

resource "aws_batch_job_definition" "test" {
  name = "%[1]s"
  type = "container"

  container_properties = <<CONTAINER_PROPERTIES
{
  "command": ["ls", "-la"],
  "image": "busybox",
  "memory": 512,
  "vcpus": 1,
  "volumes": [ ],
  "environment": [ ],
  "mountPoints": [ ],
  "ulimits": [ ]
}
CONTAINER_PROPERTIES
}
`, rName)
}

func testAccAWSCloudWatchEventTargetConfigKinesis(rName string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name                = "%[1]s"
  description         = "schedule_batch_test"
  schedule_expression = "rate(5 minutes)"
}

resource "aws_cloudwatch_event_target" "test" {
  arn      = aws_kinesis_stream.test.arn
  rule     = aws_cloudwatch_event_rule.test.id
  role_arn = aws_iam_role.test.arn

  kinesis_target {
    partition_key_path = "$.detail"
  }
}

resource "aws_iam_role" "test" {
  name = "event_%[1]s"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": "events.${data.aws_partition.current.dns_suffix}"
      }
    }
  ]
}
EOF
}

resource "aws_kinesis_stream" "test" {
  name        = "%[1]s"
  shard_count = 1
}

data "aws_partition" "current" {}
`, rName)
}

func testAccAWSCloudWatchEventTargetConfigSqs(rName string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name                = "%[1]s"
  description         = "schedule_batch_test"
  schedule_expression = "rate(5 minutes)"
}

resource "aws_cloudwatch_event_target" "test" {
  arn  = aws_sqs_queue.test.arn
  rule = aws_cloudwatch_event_rule.test.id

  sqs_target {
    message_group_id = "event_group"
  }
}

resource "aws_sqs_queue" "test" {
  name       = "%[1]s.fifo"
  fifo_queue = true
}
`, rName)
}

func testAccAWSCloudWatchEventTargetConfigInputTransformer(rName string, inputPathKeys []string) string {
	var inputPaths, inputTemplates strings.Builder

	for _, inputPath := range inputPathKeys {
		fmt.Fprintf(&inputPaths, "      %[1]s = \"$.%[1]s\"\n", inputPath)
		fmt.Fprintf(&inputTemplates, "  \"%[1]s\": <%[1]s>,\n", inputPath)
	}

	return composeConfig(
		testAccAWSCloudWatchEventTargetConfigLambdaBase(rName),
		fmt.Sprintf(`
resource "aws_cloudwatch_event_target" "test" {
  arn  = aws_lambda_function.test.arn
  rule = aws_cloudwatch_event_rule.schedule.id

  input_transformer {
    input_paths = {
      %[2]s
    }

    input_template = <<EOF
{
  "detail-type": "Scheduled Event",
  "source": "aws.events",
  %[3]s
  "detail": {}
}
EOF
  }
}

resource "aws_cloudwatch_event_rule" "schedule" {
  name        = "%[1]s"
  description = "test_input_transformer"

  schedule_expression = "rate(5 minutes)"
}
`, rName, inputPaths.String(), strings.TrimSpace(inputTemplates.String())))
}

func testAccAWSCloudWatchEventTargetConfigInputTransformerJsonString(name string) string {
	return composeConfig(
		testAccAWSCloudWatchEventTargetConfigLambdaBase(name),
		fmt.Sprintf(`
resource "aws_cloudwatch_event_target" "test" {
  arn  = aws_lambda_function.test.arn
  rule = aws_cloudwatch_event_rule.test.id

  input_transformer {
    input_paths = {
      instance = "$.detail.instance",
      status   = "$.detail.status",
    }
    input_template = "\"<instance> is in state <status>\""
  }
}

resource "aws_cloudwatch_event_rule" "test" {
  name        = %[1]q
  description = "test_input_transformer"

  schedule_expression = "rate(5 minutes)"
}
`, name))
}

func testAccAWSCloudWatchEventTargetConfigLambdaBase(name string) string {
	return fmt.Sprintf(`
resource "aws_lambda_function" "test" {
  function_name    = %[1]q
  filename         = "test-fixtures/lambdatest.zip"
  source_code_hash = filebase64sha256("test-fixtures/lambdatest.zip")
  role             = aws_iam_role.test.arn
  handler          = "exports.example"
  runtime          = "nodejs12.x"
}

resource "aws_iam_role" "test" {
  name = %[1]q

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

data "aws_partition" "current" {}
`, name)
}

func testAccAWSCloudWatchEventTargetPartnerEventBusConfig(rName, eventBusName string) string {
	return fmt.Sprintf(`
resource "aws_cloudwatch_event_rule" "test" {
  name           = %[1]q
  event_bus_name = %[2]q

  event_pattern = <<PATTERN
{
  "source": ["aws.ec2"]
}
PATTERN
}

resource "aws_cloudwatch_event_target" "test" {
  rule           = aws_cloudwatch_event_rule.test.name
  event_bus_name = aws_cloudwatch_event_rule.test.event_bus_name
  target_id      = %[1]q
  arn            = aws_sns_topic.test.arn
}

resource "aws_sns_topic" "test" {
  name = %[1]q
}
`, rName, eventBusName)
}
