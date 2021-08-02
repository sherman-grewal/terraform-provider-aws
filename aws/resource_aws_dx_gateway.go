package aws

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/directconnect"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/service/directconnect/finder"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/service/directconnect/waiter"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/tfresource"
)

func resourceAwsDxGateway() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsDxGatewayCreate,
		Read:   resourceAwsDxGatewayRead,
		Delete: resourceAwsDxGatewayDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"amazon_side_asn": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateAmazonSideAsn,
			},

			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"owner_account_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},
	}
}

func resourceAwsDxGatewayCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dxconn

	name := d.Get("name").(string)
	input := &directconnect.CreateDirectConnectGatewayInput{
		DirectConnectGatewayName: aws.String(name),
	}

	if v, ok := d.Get("amazon_side_asn").(string); ok && v != "" {
		if v, err := strconv.ParseInt(v, 10, 64); err == nil {
			input.AmazonSideAsn = aws.Int64(v)
		}
	}

	log.Printf("[DEBUG] Creating Direct Connect Gateway: %s", input)
	resp, err := conn.CreateDirectConnectGateway(input)

	if err != nil {
		return fmt.Errorf("error creating Direct Connect Gateway (%s): %w", name, err)
	}

	d.SetId(aws.StringValue(resp.DirectConnectGateway.DirectConnectGatewayId))

	if _, err := waiter.GatewayCreated(conn, d.Id(), d.Timeout(schema.TimeoutCreate)); err != nil {
		return fmt.Errorf("error waiting for Direct Connect Gateway (%s) to create: %w", d.Id(), err)
	}

	return resourceAwsDxGatewayRead(d, meta)
}

func resourceAwsDxGatewayRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dxconn

	output, err := finder.GatewayByID(conn, d.Id())

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] Direct Connect Gateway (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("error reading Direct Connect Gateway (%s): %w", d.Id(), err)
	}

	d.Set("amazon_side_asn", strconv.FormatInt(aws.Int64Value(output.AmazonSideAsn), 10))
	d.Set("name", output.DirectConnectGatewayName)
	d.Set("owner_account_id", output.OwnerAccount)

	return nil
}

func resourceAwsDxGatewayDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).dxconn

	log.Printf("[DEBUG] Deleting Direct Connect Gateway: %s", d.Id())
	_, err := conn.DeleteDirectConnectGateway(&directconnect.DeleteDirectConnectGatewayInput{
		DirectConnectGatewayId: aws.String(d.Id()),
	})

	if tfawserr.ErrMessageContains(err, directconnect.ErrCodeClientException, "does not exist") {
		return nil
	}

	if err != nil {
		return fmt.Errorf("error deleting Direct Connect Gateway (%s): %w", d.Id(), err)
	}

	if _, err := waiter.GatewayDeleted(conn, d.Id(), d.Timeout(schema.TimeoutDelete)); err != nil {
		return fmt.Errorf("error waiting for Direct Connect Gateway (%s) to delete: %w", d.Id(), err)
	}

	return nil
}
