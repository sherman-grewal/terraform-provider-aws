package aws

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/appconfig"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/keyvaluetags"
)

func resourceAwsAppconfigEnvironment() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsAppconfigEnvironmentCreate,
		Read:   resourceAwsAppconfigEnvironmentRead,
		Update: resourceAwsAppconfigEnvironmentUpdate,
		Delete: resourceAwsAppconfigEnvironmentDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"application_id": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile(`[a-z0-9]{4,7}`), ""),
			},
			"environment_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 64),
			},
			"description": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(0, 1024),
			},
			"monitor": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 5,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"alarm_arn": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.All(
								validation.StringLenBetween(1, 2048),
								validateArn,
							),
						},
						"alarm_role_arn": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validateArn,
						},
					},
				},
			},
			"state": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tags":     tagsSchema(),
			"tags_all": tagsSchemaComputed(),
		},
		CustomizeDiff: SetTagsDiff,
	}
}

func resourceAwsAppconfigEnvironmentCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).appconfigconn
	defaultTagsConfig := meta.(*AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(keyvaluetags.New(d.Get("tags").(map[string]interface{})))

	appId := d.Get("application_id").(string)

	input := &appconfig.CreateEnvironmentInput{
		Name:          aws.String(d.Get("name").(string)),
		ApplicationId: aws.String(appId),
		Tags:          tags.IgnoreAws().AppconfigTags(),
	}

	if v, ok := d.GetOk("description"); ok {
		input.Description = aws.String(v.(string))
	}

	if v, ok := d.GetOk("monitor"); ok && v.(*schema.Set).Len() > 0 {
		input.Monitors = expandAppconfigEnvironmentMonitors(v.(*schema.Set).List())
	}

	environment, err := conn.CreateEnvironment(input)

	if err != nil {
		return fmt.Errorf("error creating AppConfig Environment for Application (%s): %w", appId, err)
	}

	if environment == nil {
		return fmt.Errorf("error creating AppConfig Environment for Application (%s): empty response", appId)
	}

	d.Set("environment_id", environment.Id)
	d.SetId(fmt.Sprintf("%s:%s", aws.StringValue(environment.Id), aws.StringValue(environment.ApplicationId)))

	return resourceAwsAppconfigEnvironmentRead(d, meta)
}

func resourceAwsAppconfigEnvironmentRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).appconfigconn
	defaultTagsConfig := meta.(*AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*AWSClient).IgnoreTagsConfig

	envID, appID, err := resourceAwsAppconfigEnvironmentParseID(d.Id())

	if err != nil {
		return err
	}

	input := &appconfig.GetEnvironmentInput{
		ApplicationId: aws.String(appID),
		EnvironmentId: aws.String(envID),
	}

	output, err := conn.GetEnvironment(input)

	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, appconfig.ErrCodeResourceNotFoundException) {
		log.Printf("[WARN] Appconfig Environment (%s) for Application (%s) not found, removing from state", envID, appID)
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("error getting AppConfig Environment (%s) for Application (%s): %w", envID, appID, err)
	}

	if output == nil {
		return fmt.Errorf("error getting AppConfig Environment (%s) for Application (%s): empty response", envID, appID)
	}

	d.Set("application_id", output.ApplicationId)
	d.Set("environment_id", output.Id)
	d.Set("description", output.Description)
	d.Set("name", output.Name)
	d.Set("state", output.State)

	if err := d.Set("monitor", flattenAwsAppconfigEnvironmentMonitors(output.Monitors)); err != nil {
		return fmt.Errorf("error setting monitor: %w", err)
	}

	arn := arn.ARN{
		AccountID: meta.(*AWSClient).accountid,
		Partition: meta.(*AWSClient).partition,
		Region:    meta.(*AWSClient).region,
		Resource:  fmt.Sprintf("application/%s/environment/%s", appID, envID),
		Service:   "appconfig",
	}.String()

	d.Set("arn", arn)

	tags, err := keyvaluetags.AppconfigListTags(conn, arn)

	if err != nil {
		return fmt.Errorf("error listing tags for AppConfig Environment (%s): %s", d.Id(), err)
	}

	tags = tags.IgnoreAws().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return fmt.Errorf("error setting tags: %w", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return fmt.Errorf("error setting tags_all: %w", err)
	}

	return nil
}

func resourceAwsAppconfigEnvironmentUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).appconfigconn

	if d.HasChangesExcept("tags", "tags_all") {
		envID, appID, err := resourceAwsAppconfigEnvironmentParseID(d.Id())

		if err != nil {
			return err
		}

		updateInput := &appconfig.UpdateEnvironmentInput{
			EnvironmentId: aws.String(envID),
			ApplicationId: aws.String(appID),
		}

		if d.HasChange("description") {
			updateInput.Description = aws.String(d.Get("description").(string))
		}

		if d.HasChange("name") {
			updateInput.Name = aws.String(d.Get("name").(string))
		}

		if d.HasChange("monitor") {
			updateInput.Monitors = expandAppconfigEnvironmentMonitors(d.Get("monitor").(*schema.Set).List())
		}

		_, err = conn.UpdateEnvironment(updateInput)

		if err != nil {
			return fmt.Errorf("error updating AppConfig Environment (%s) for Application (%s): %w", envID, appID, err)
		}
	}

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")
		if err := keyvaluetags.AppconfigUpdateTags(conn, d.Get("arn").(string), o, n); err != nil {
			return fmt.Errorf("error updating AppConfig Environment (%s) tags: %w", d.Get("arn").(string), err)
		}
	}

	return resourceAwsAppconfigEnvironmentRead(d, meta)
}

func resourceAwsAppconfigEnvironmentDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).appconfigconn

	envID, appID, err := resourceAwsAppconfigEnvironmentParseID(d.Id())

	if err != nil {
		return err
	}

	input := &appconfig.DeleteEnvironmentInput{
		EnvironmentId: aws.String(envID),
		ApplicationId: aws.String(appID),
	}

	_, err = conn.DeleteEnvironment(input)

	if tfawserr.ErrCodeEquals(err, appconfig.ErrCodeResourceNotFoundException) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("error deleting Appconfig Environment (%s) for Application (%s): %w", envID, appID, err)
	}

	return nil
}

func resourceAwsAppconfigEnvironmentParseID(id string) (string, string, error) {
	parts := strings.Split(id, ":")

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unexpected format of ID (%q), expected EnvironmentID:ApplicationID", id)
	}

	return parts[0], parts[1], nil
}

func expandAppconfigEnvironmentMonitor(tfMap map[string]interface{}) *appconfig.Monitor {
	if tfMap == nil {
		return nil
	}

	monitor := &appconfig.Monitor{}

	if v, ok := tfMap["alarm_arn"].(string); ok && v != "" {
		monitor.AlarmArn = aws.String(v)
	}

	if v, ok := tfMap["alarm_role_arn"].(string); ok && v != "" {
		monitor.AlarmRoleArn = aws.String(v)
	}

	return monitor
}

func expandAppconfigEnvironmentMonitors(tfList []interface{}) []*appconfig.Monitor {
	// AppConfig API requires a 0 length slice instead of a nil value
	// when updating from N monitors to 0/nil monitors
	monitors := make([]*appconfig.Monitor, 0)

	for _, tfMapRaw := range tfList {
		tfMap, ok := tfMapRaw.(map[string]interface{})

		if !ok {
			continue
		}

		monitor := expandAppconfigEnvironmentMonitor(tfMap)

		if monitor == nil {
			continue
		}

		monitors = append(monitors, monitor)
	}

	return monitors
}

func flattenAwsAppconfigEnvironmentMonitor(monitor *appconfig.Monitor) map[string]interface{} {
	if monitor == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := monitor.AlarmArn; v != nil {
		tfMap["alarm_arn"] = aws.StringValue(v)
	}

	if v := monitor.AlarmRoleArn; v != nil {
		tfMap["alarm_role_arn"] = aws.StringValue(v)
	}

	return tfMap
}

func flattenAwsAppconfigEnvironmentMonitors(monitors []*appconfig.Monitor) []interface{} {
	if len(monitors) == 0 {
		return nil
	}

	var tfList []interface{}

	for _, monitor := range monitors {
		if monitor == nil {
			continue
		}

		tfList = append(tfList, flattenAwsAppconfigEnvironmentMonitor(monitor))
	}

	return tfList
}
