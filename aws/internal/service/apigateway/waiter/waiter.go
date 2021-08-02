package waiter

import (
	"time"

	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

const (
	// Maximum amount of time for VpcLink to delete
	ApiGatewayVpcLinkDeleteTimeout = 20 * time.Minute
)

func ApiGatewayVpcLinkDeleted(conn *apigateway.APIGateway, vpcLinkId string) error {
	stateConf := resource.StateChangeConf{
		Pending: []string{apigateway.VpcLinkStatusPending,
			apigateway.VpcLinkStatusAvailable,
			apigateway.VpcLinkStatusDeleting},
		Target:     []string{""},
		Timeout:    ApiGatewayVpcLinkDeleteTimeout,
		MinTimeout: 1 * time.Second,
		Refresh:    apiGatewayVpcLinkStatus(conn, vpcLinkId),
	}

	_, err := stateConf.WaitForState()

	return err
}
