package datasync_test

import (
	"testing"

	tfdatasync "github.com/terraform-providers/terraform-provider-aws/aws/internal/service/datasync"
)

func TestSubdirectoryFromLocationURI(t *testing.T) {
	testCases := []struct {
		TestName             string
		InputURI             string
		ExpectedError        bool
		ExpectedSubdirectory string
	}{
		{
			TestName:      "empty URI",
			InputURI:      "",
			ExpectedError: true,
		},
		{
			TestName:      "invalid URI scheme",
			InputURI:      "test://testing/",
			ExpectedError: true,
		},
		{
			TestName:      "S3 bucket URI no bucket name (1)",
			InputURI:      "s3://",
			ExpectedError: true,
		},
		{
			TestName:      "S3 bucket URI no bucket name (2)",
			InputURI:      "s3:///",
			ExpectedError: true,
		},
		{
			TestName:             "S3 bucket URI top level",
			InputURI:             "s3://bucket/",
			ExpectedSubdirectory: "/",
		},
		{
			TestName:             "S3 bucket URI one level",
			InputURI:             "s3://bucket/my-folder-1/",
			ExpectedSubdirectory: "/my-folder-1/",
		},
		{
			TestName:             "S3 bucket URI two levels",
			InputURI:             "s3://bucket/my-folder-1/my-folder-2",
			ExpectedSubdirectory: "/my-folder-1/my-folder-2",
		},
		{
			TestName:             "S3 Outposts ARN URI top level",
			InputURI:             "s3://arn:aws:s3-outposts:eu-west-3:123456789012:outpost/op-YYYYYYYYYY/accesspoint/my-access-point/",
			ExpectedSubdirectory: "/",
		},
		{
			TestName:             "S3 Outposts ARN URI one level",
			InputURI:             "s3://arn:aws:s3-outposts:eu-west-3:123456789012:outpost/op-YYYYYYYYYY/accesspoint/my-access-point/my-folder-1/",
			ExpectedSubdirectory: "/my-folder-1/",
		},
		{
			TestName:             "S3 Outposts ARN URI two levels",
			InputURI:             "s3://arn:aws:s3-outposts:eu-west-3:123456789012:outpost/op-YYYYYYYYYY/accesspoint/my-access-point/my-folder-1/my-folder-2",
			ExpectedSubdirectory: "/my-folder-1/my-folder-2",
		},
		{
			TestName:             "EFS URI top level",
			InputURI:             "efs://us-west-2.fs-abcdef01/",
			ExpectedSubdirectory: "/",
		},
		{
			TestName:             "EFS URI one level",
			InputURI:             "efs://us-west-2.fs-abcdef01/my-folder-1/",
			ExpectedSubdirectory: "/my-folder-1/",
		},
		{
			TestName:             "EFS URI two levels",
			InputURI:             "efs://us-west-2.fs-abcdef01/my-folder-1/my-folder-2",
			ExpectedSubdirectory: "/my-folder-1/my-folder-2",
		},
		{
			TestName:             "NFS URI top level",
			InputURI:             "nfs://example.com/",
			ExpectedSubdirectory: "/",
		},
		{
			TestName:             "NFS URI one level",
			InputURI:             "nfs://example.com/my-folder-1/",
			ExpectedSubdirectory: "/my-folder-1/",
		},
		{
			TestName:             "NFS URI two levels",
			InputURI:             "nfs://example.com/my-folder-1/my-folder-2",
			ExpectedSubdirectory: "/my-folder-1/my-folder-2",
		},
		{
			TestName:             "SMB URI top level",
			InputURI:             "smb://192.168.1.1/",
			ExpectedSubdirectory: "/",
		},
		{
			TestName:             "SMB URI one level",
			InputURI:             "smb://192.168.1.1/my-folder-1/",
			ExpectedSubdirectory: "/my-folder-1/",
		},
		{
			TestName:             "SMB URI two levels",
			InputURI:             "smb://192.168.1.1/my-folder-1/my-folder-2",
			ExpectedSubdirectory: "/my-folder-1/my-folder-2",
		},
		{
			TestName:             "FSx Windows URI top level",
			InputURI:             "fsxw://us-west-2.fs-abcdef012345678901/",
			ExpectedSubdirectory: "/",
		},
		{
			TestName:             "FSx Windows URI one level",
			InputURI:             "fsxw://us-west-2.fs-abcdef012345678901/my-folder-1/",
			ExpectedSubdirectory: "/my-folder-1/",
		},
		{
			TestName:             "FSx Windows URI two levels",
			InputURI:             "fsxw://us-west-2.fs-abcdef012345678901/my-folder-1/my-folder-2",
			ExpectedSubdirectory: "/my-folder-1/my-folder-2",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.TestName, func(t *testing.T) {
			got, err := tfdatasync.SubdirectoryFromLocationURI(testCase.InputURI)

			if err == nil && testCase.ExpectedError {
				t.Fatalf("expected error")
			}

			if err != nil && !testCase.ExpectedError {
				t.Fatalf("unexpected error: %s", err)
			}

			if got != testCase.ExpectedSubdirectory {
				t.Errorf("got %s, expected %s", got, testCase.ExpectedSubdirectory)
			}
		})
	}
}
