package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceAwsEc2InstanceTypeOfferings() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceAwsEc2InstanceTypeOfferingsRead,

		Schema: map[string]*schema.Schema{
			"filter": dataSourceFiltersSchema(),
			"instance_types": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"locations": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"location_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(ec2.LocationType_Values(), false),
			},
			"location_types": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func dataSourceAwsEc2InstanceTypeOfferingsRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).ec2conn

	input := &ec2.DescribeInstanceTypeOfferingsInput{}

	if v, ok := d.GetOk("filter"); ok {
		input.Filters = buildAwsDataSourceFilters(v.(*schema.Set))
	}

	if v, ok := d.GetOk("location_type"); ok {
		input.LocationType = aws.String(v.(string))
	}

	var instanceTypes []string
	var locations []string
	var locationTypes []string

	err := conn.DescribeInstanceTypeOfferingsPages(input, func(page *ec2.DescribeInstanceTypeOfferingsOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, instanceTypeOffering := range page.InstanceTypeOfferings {
			if instanceTypeOffering == nil {
				continue
			}

			instanceTypes = append(instanceTypes, aws.StringValue(instanceTypeOffering.InstanceType))
			locations = append(locations, aws.StringValue(instanceTypeOffering.Location))
			locationTypes = append(locationTypes, aws.StringValue(instanceTypeOffering.LocationType))
		}

		return !lastPage
	})

	if err != nil {
		return fmt.Errorf("error reading EC2 Instance Type Offerings: %w", err)
	}

	if err := d.Set("instance_types", instanceTypes); err != nil {
		return fmt.Errorf("error setting instance_types: %w", err)
	}
	if err := d.Set("locations", locations); err != nil {
		return fmt.Errorf("error setting locations: %w", err)
	}
	if err := d.Set("location_types", locationTypes); err != nil {
		return fmt.Errorf("error setting location_types: %w", err)
	}

	d.SetId(meta.(*AWSClient).region)

	return nil
}
