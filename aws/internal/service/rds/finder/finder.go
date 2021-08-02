package finder

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	tfrds "github.com/terraform-providers/terraform-provider-aws/aws/internal/service/rds"
)

// DBProxyTarget returns matching DBProxyTarget.
func DBProxyTarget(conn *rds.RDS, dbProxyName, targetGroupName, targetType, rdsResourceId string) (*rds.DBProxyTarget, error) {
	input := &rds.DescribeDBProxyTargetsInput{
		DBProxyName:     aws.String(dbProxyName),
		TargetGroupName: aws.String(targetGroupName),
	}
	var dbProxyTarget *rds.DBProxyTarget

	err := conn.DescribeDBProxyTargetsPages(input, func(page *rds.DescribeDBProxyTargetsOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, target := range page.Targets {
			if aws.StringValue(target.Type) == targetType && aws.StringValue(target.RdsResourceId) == rdsResourceId {
				dbProxyTarget = target
				return false
			}
		}

		return !lastPage
	})

	return dbProxyTarget, err
}

// DBProxyEndpoint returns matching DBProxyEndpoint.
func DBProxyEndpoint(conn *rds.RDS, id string) (*rds.DBProxyEndpoint, error) {
	dbProxyName, dbProxyEndpointName, err := tfrds.ResourceAwsDbProxyEndpointParseID(id)
	if err != nil {
		return nil, err
	}

	input := &rds.DescribeDBProxyEndpointsInput{
		DBProxyName:         aws.String(dbProxyName),
		DBProxyEndpointName: aws.String(dbProxyEndpointName),
	}
	var dbProxyEndpoint *rds.DBProxyEndpoint

	err = conn.DescribeDBProxyEndpointsPages(input, func(page *rds.DescribeDBProxyEndpointsOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, endpoint := range page.DBProxyEndpoints {
			if aws.StringValue(endpoint.DBProxyEndpointName) == dbProxyEndpointName &&
				aws.StringValue(endpoint.DBProxyName) == dbProxyName {
				dbProxyEndpoint = endpoint
				return false
			}
		}

		return !lastPage
	})

	return dbProxyEndpoint, err
}

func DBClusterRoleByDBClusterIDAndRoleARN(conn *rds.RDS, dbClusterID, roleARN string) (*rds.DBClusterRole, error) {
	dbCluster, err := DBClusterByID(conn, dbClusterID)

	if err != nil {
		return nil, err
	}

	for _, associatedRole := range dbCluster.AssociatedRoles {
		if aws.StringValue(associatedRole.RoleArn) == roleARN {
			if status := aws.StringValue(associatedRole.Status); status == tfrds.DBClusterRoleStatusDeleted {
				return nil, &resource.NotFoundError{
					Message: status,
				}
			}

			return associatedRole, nil
		}
	}

	return nil, &resource.NotFoundError{}
}

func DBClusterByID(conn *rds.RDS, id string) (*rds.DBCluster, error) {
	input := &rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(id),
	}

	output, err := conn.DescribeDBClusters(input)

	if tfawserr.ErrCodeEquals(err, rds.ErrCodeDBClusterNotFoundFault) {
		return nil, &resource.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}

	if output == nil || len(output.DBClusters) == 0 || output.DBClusters[0] == nil {
		return nil, &resource.NotFoundError{
			Message:     "Empty result",
			LastRequest: input,
		}
	}

	dbCluster := output.DBClusters[0]

	// Eventual consistency check.
	if aws.StringValue(dbCluster.DBClusterIdentifier) != id {
		return nil, &resource.NotFoundError{
			LastRequest: input,
		}
	}

	return dbCluster, nil
}
