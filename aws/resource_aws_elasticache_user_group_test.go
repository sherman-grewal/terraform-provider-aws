package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/service/elasticache/finder"
)

func TestAccAWSElasticacheUserGroup_basic(t *testing.T) {
	var userGroup elasticache.UserGroup
	rName := acctest.RandomWithPrefix("tf-acc")
	resourceName := "aws_elasticache_user_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, elasticache.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSElasticacheUserGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSElasticacheUserGroupConfigBasic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSElasticacheUserGroupExists(resourceName, &userGroup),
					resource.TestCheckResourceAttr(resourceName, "user_ids.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "user_group_id", rName),
					resource.TestCheckResourceAttr(resourceName, "engine", "redis"),
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

func TestAccAWSElasticacheUserGroup_update(t *testing.T) {
	var userGroup elasticache.UserGroup
	rName := acctest.RandomWithPrefix("tf-acc")
	resourceName := "aws_elasticache_user_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, elasticache.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSElasticacheUserGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSElasticacheUserGroupConfigBasic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSElasticacheUserGroupExists(resourceName, &userGroup),
					resource.TestCheckResourceAttr(resourceName, "user_ids.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "user_group_id", rName),
					resource.TestCheckResourceAttr(resourceName, "engine", "redis"),
				),
			},
			{
				Config: testAccAWSElasticacheUserGroupConfigMultiple(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSElasticacheUserGroupExists(resourceName, &userGroup),
					resource.TestCheckResourceAttr(resourceName, "user_ids.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "user_group_id", rName),
					resource.TestCheckResourceAttr(resourceName, "engine", "redis"),
				),
			},
			{
				Config: testAccAWSElasticacheUserGroupConfigBasic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSElasticacheUserGroupExists(resourceName, &userGroup),
					resource.TestCheckResourceAttr(resourceName, "user_ids.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "user_group_id", rName),
					resource.TestCheckResourceAttr(resourceName, "engine", "redis"),
				),
			},
		},
	})
}

func TestAccAWSElasticacheUserGroup_tags(t *testing.T) {
	var userGroup elasticache.UserGroup
	rName := acctest.RandomWithPrefix("tf-acc")
	resourceName := "aws_elasticache_user_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, elasticache.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSElasticacheUserGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSElasticacheUserGroupConfigTags(rName, "tagKey", "tagVal"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSElasticacheUserGroupExists(resourceName, &userGroup),
					resource.TestCheckResourceAttr(resourceName, "user_ids.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "user_group_id", rName),
					resource.TestCheckResourceAttr(resourceName, "engine", "redis"),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.tagKey", "tagVal"),
				),
			},
			{
				Config: testAccAWSElasticacheUserGroupConfigTags(rName, "tagKey", "tagVal2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSElasticacheUserGroupExists(resourceName, &userGroup),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.tagKey", "tagVal2"),
				),
			},
			{
				Config: testAccAWSElasticacheUserGroupConfigBasic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSElasticacheUserGroupExists(resourceName, &userGroup),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
				),
			},
		},
	})
}

func TestAccAWSElasticacheUserGroup_disappears(t *testing.T) {
	var userGroup elasticache.UserGroup
	rName := acctest.RandomWithPrefix("tf-acc")
	resourceName := "aws_elasticache_user_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, elasticache.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSNeptuneClusterEndpointDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSElasticacheUserGroupConfigBasic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSElasticacheUserGroupExists(resourceName, &userGroup),
					testAccCheckResourceDisappears(testAccProvider, resourceAwsElasticacheUserGroup(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckAWSElasticacheUserGroupDestroy(s *terraform.State) error {
	return testAccCheckAWSElasticacheUserGroupDestroyWithProvider(s, testAccProvider)
}

func testAccCheckAWSElasticacheUserGroupDestroyWithProvider(s *terraform.State, provider *schema.Provider) error {
	conn := provider.Meta().(*AWSClient).elasticacheconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_elasticache_user_group" {
			continue
		}

		_, err := finder.ElastiCacheUserGroupById(conn, rs.Primary.ID)
		if err != nil {
			if isAWSErr(err, elasticache.ErrCodeUserGroupNotFoundFault, "") {
				return nil
			}
		}

		return err
	}

	return nil
}

func testAccCheckAWSElasticacheUserGroupExists(n string, v *elasticache.UserGroup) resource.TestCheckFunc {
	return testAccCheckAWSElasticacheUserGroupExistsWithProvider(n, v, func() *schema.Provider { return testAccProvider })
}

func testAccCheckAWSElasticacheUserGroupExistsWithProvider(n string, v *elasticache.UserGroup, providerF func() *schema.Provider) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No ElastiCache User Group ID is set")
		}

		provider := providerF()
		conn := provider.Meta().(*AWSClient).elasticacheconn
		resp, err := finder.ElastiCacheUserGroupById(conn, rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("ElastiCache User Group (%s) not found: %w", rs.Primary.ID, err)
		}

		*v = *resp

		return nil
	}
}

func testAccAWSElasticacheUserGroupConfigBasic(rName string) string {
	return composeConfig(testAccAvailableAZsNoOptInConfig(), fmt.Sprintf(`
resource "aws_elasticache_user" "test" {
  user_id       = %[1]q
  user_name     = "default"
  access_string = "on ~app::* -@all +@read +@hash +@bitmap +@geo -setbit -bitfield -hset -hsetnx -hmset -hincrby -hincrbyfloat -hdel -bitop -geoadd -georadius -georadiusbymember"
  engine        = "REDIS"
  passwords     = ["password123456789"]
}

resource "aws_elasticache_user_group" "test" {
  user_group_id = %[1]q
  engine        = "REDIS"
  user_ids      = [aws_elasticache_user.test.user_id]
}
`, rName))
}

func testAccAWSElasticacheUserGroupConfigMultiple(rName string) string {
	return composeConfig(testAccAvailableAZsNoOptInConfig(), fmt.Sprintf(`
resource "aws_elasticache_user" "test" {
  user_id       = %[1]q
  user_name     = "default"
  access_string = "on ~app::* -@all +@read +@hash +@bitmap +@geo -setbit -bitfield -hset -hsetnx -hmset -hincrby -hincrbyfloat -hdel -bitop -geoadd -georadius -georadiusbymember"
  engine        = "REDIS"
  passwords     = ["password123456789"]
}

resource "aws_elasticache_user" "test2" {
  user_id       = "%[1]s-2"
  user_name     = "username1"
  access_string = "on ~app::* -@all +@read +@hash +@bitmap +@geo -setbit -bitfield -hset -hsetnx -hmset -hincrby -hincrbyfloat -hdel -bitop -geoadd -georadius -georadiusbymember"
  engine        = "REDIS"
  passwords     = ["password123456789"]
}

resource "aws_elasticache_user_group" "test" {
  user_group_id = %[1]q
  engine        = "REDIS"
  user_ids      = [aws_elasticache_user.test.user_id, aws_elasticache_user.test2.user_id]
}
`, rName))
}

func testAccAWSElasticacheUserGroupConfigTags(rName, tagKey, tagValue string) string {
	return composeConfig(testAccAvailableAZsNoOptInConfig(), fmt.Sprintf(`
resource "aws_elasticache_user" "test" {
  user_id       = %[1]q
  user_name     = "default"
  access_string = "on ~app::* -@all +@read +@hash +@bitmap +@geo -setbit -bitfield -hset -hsetnx -hmset -hincrby -hincrbyfloat -hdel -bitop -geoadd -georadius -georadiusbymember"
  engine        = "REDIS"
  passwords     = ["password123456789"]
}

resource "aws_elasticache_user_group" "test" {
  user_group_id = %[1]q
  engine        = "REDIS"
  user_ids      = [aws_elasticache_user.test.user_id]

  tags = {
    %[2]s = %[3]q
  }
}
`, rName, tagKey, tagValue))
}
