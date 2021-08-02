package aws

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/guardduty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/keyvaluetags"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/service/guardduty/waiter"
)

func resourceAwsGuardDutyDetector() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsGuardDutyDetectorCreate,
		Read:   resourceAwsGuardDutyDetectorRead,
		Update: resourceAwsGuardDutyDetectorUpdate,
		Delete: resourceAwsGuardDutyDetectorDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"account_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"datasources": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"s3_logs": {
							Type:     schema.TypeList,
							Optional: true,
							Computed: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"enable": {
										Type:     schema.TypeBool,
										Required: true,
									},
								},
							},
						},
					},
				},
			},

			"enable": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},

			// finding_publishing_frequency is marked as Computed:true since
			// GuardDuty member accounts inherit setting from master account
			"finding_publishing_frequency": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"tags":     tagsSchema(),
			"tags_all": tagsSchemaComputed(),
		},

		CustomizeDiff: SetTagsDiff,
	}
}

func resourceAwsGuardDutyDetectorCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).guarddutyconn
	defaultTagsConfig := meta.(*AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(keyvaluetags.New(d.Get("tags").(map[string]interface{})))

	input := guardduty.CreateDetectorInput{
		Enable: aws.Bool(d.Get("enable").(bool)),
	}

	if v, ok := d.GetOk("finding_publishing_frequency"); ok {
		input.FindingPublishingFrequency = aws.String(v.(string))
	}

	if v, ok := d.GetOk("datasources"); ok && len(v.([]interface{})) > 0 && v.([]interface{})[0] != nil {
		input.DataSources = expandGuardDutyDataSourceConfigurations(v.([]interface{})[0].(map[string]interface{}))
	}

	if len(tags) > 0 {
		input.Tags = tags.IgnoreAws().GuarddutyTags()
	}

	log.Printf("[DEBUG] Creating GuardDuty Detector: %s", input)
	output, err := conn.CreateDetector(&input)
	if err != nil {
		return fmt.Errorf("Creating GuardDuty Detector failed: %s", err.Error())
	}
	d.SetId(aws.StringValue(output.DetectorId))

	return resourceAwsGuardDutyDetectorRead(d, meta)
}

func resourceAwsGuardDutyDetectorRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).guarddutyconn
	defaultTagsConfig := meta.(*AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*AWSClient).IgnoreTagsConfig

	input := guardduty.GetDetectorInput{
		DetectorId: aws.String(d.Id()),
	}

	log.Printf("[DEBUG] Reading GuardDuty Detector: %s", input)
	gdo, err := conn.GetDetector(&input)
	if err != nil {
		if isAWSErr(err, guardduty.ErrCodeBadRequestException, "The request is rejected because the input detectorId is not owned by the current account.") {
			log.Printf("[WARN] GuardDuty detector %q not found, removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Reading GuardDuty Detector '%s' failed: %s", d.Id(), err.Error())
	}

	arn := arn.ARN{
		Partition: meta.(*AWSClient).partition,
		Region:    meta.(*AWSClient).region,
		Service:   "guardduty",
		AccountID: meta.(*AWSClient).accountid,
		Resource:  fmt.Sprintf("detector/%s", d.Id()),
	}.String()
	d.Set("arn", arn)

	d.Set("account_id", meta.(*AWSClient).accountid)

	if gdo.DataSources != nil {
		if err := d.Set("datasources", []interface{}{flattenGuardDutyDataSourceConfigurationsResult(gdo.DataSources)}); err != nil {
			return fmt.Errorf("error setting datasources: %w", err)
		}
	} else {
		d.Set("datasources", nil)
	}

	d.Set("enable", aws.StringValue(gdo.Status) == guardduty.DetectorStatusEnabled)
	d.Set("finding_publishing_frequency", gdo.FindingPublishingFrequency)

	tags := keyvaluetags.GuarddutyKeyValueTags(gdo.Tags).IgnoreAws().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return fmt.Errorf("error setting tags: %w", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return fmt.Errorf("error setting tags_all: %w", err)
	}

	return nil
}

func resourceAwsGuardDutyDetectorUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).guarddutyconn

	if d.HasChangesExcept("tags", "tags_all") {
		input := guardduty.UpdateDetectorInput{
			DetectorId:                 aws.String(d.Id()),
			Enable:                     aws.Bool(d.Get("enable").(bool)),
			FindingPublishingFrequency: aws.String(d.Get("finding_publishing_frequency").(string)),
		}

		if d.HasChange("datasources") {
			input.DataSources = expandGuardDutyDataSourceConfigurations(d.Get("datasources").([]interface{})[0].(map[string]interface{}))
		}

		log.Printf("[DEBUG] Update GuardDuty Detector: %s", input)
		_, err := conn.UpdateDetector(&input)
		if err != nil {
			return fmt.Errorf("Updating GuardDuty Detector '%s' failed: %s", d.Id(), err.Error())
		}
	}

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")

		if err := keyvaluetags.GuarddutyUpdateTags(conn, d.Get("arn").(string), o, n); err != nil {
			return fmt.Errorf("error updating GuardDuty Detector (%s) tags: %s", d.Get("arn").(string), err)
		}
	}

	return resourceAwsGuardDutyDetectorRead(d, meta)
}

func resourceAwsGuardDutyDetectorDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).guarddutyconn

	input := &guardduty.DeleteDetectorInput{
		DetectorId: aws.String(d.Id()),
	}

	err := resource.Retry(waiter.MembershipPropagationTimeout, func() *resource.RetryError {
		_, err := conn.DeleteDetector(input)

		if isAWSErr(err, guardduty.ErrCodeBadRequestException, "cannot delete detector while it has invited or associated members") {
			return resource.RetryableError(err)
		}

		if err != nil {
			return resource.NonRetryableError(err)
		}

		return nil
	})

	if isResourceTimeoutError(err) {
		_, err = conn.DeleteDetector(input)
	}

	if err != nil {
		return fmt.Errorf("error deleting GuardDuty Detector (%s): %w", d.Id(), err)
	}

	return nil
}

func expandGuardDutyDataSourceConfigurations(tfMap map[string]interface{}) *guardduty.DataSourceConfigurations {
	if tfMap == nil {
		return nil
	}

	apiObject := &guardduty.DataSourceConfigurations{}

	if v, ok := tfMap["s3_logs"].([]interface{}); ok && len(v) > 0 {
		apiObject.S3Logs = expandGuardDutyS3LogsConfiguration(v[0].(map[string]interface{}))
	}

	return apiObject
}

func expandGuardDutyS3LogsConfiguration(tfMap map[string]interface{}) *guardduty.S3LogsConfiguration {
	if tfMap == nil {
		return nil
	}

	apiObject := &guardduty.S3LogsConfiguration{}

	if v, ok := tfMap["enable"].(bool); ok {
		apiObject.Enable = aws.Bool(v)
	}

	return apiObject
}

func flattenGuardDutyDataSourceConfigurationsResult(apiObject *guardduty.DataSourceConfigurationsResult) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.S3Logs; v != nil {
		tfMap["s3_logs"] = []interface{}{flattenGuardDutyS3LogsConfigurationResult(v)}
	}

	return tfMap
}

func flattenGuardDutyS3LogsConfigurationResult(apiObject *guardduty.S3LogsConfigurationResult) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.Status; v != nil {
		tfMap["enable"] = aws.StringValue(v) == guardduty.DataSourceStatusEnabled
	}

	return tfMap
}
