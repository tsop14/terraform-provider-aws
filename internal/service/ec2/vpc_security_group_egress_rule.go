package ec2

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func init() {
	registerFrameworkResourceFactory(newResourceSecurityGroupEgressRule)
}

// newResourceSecurityGroupEgressRule instantiates a new Resource for the aws_vpc_security_group_egress_rule resource.
func newResourceSecurityGroupEgressRule(context.Context) (resource.ResourceWithConfigure, error) {
	r := &resourceSecurityGroupEgressRule{}
	r.create = r.createSecurityGroupRule
	r.delete = r.deleteSecurityGroupRule
	r.findByID = r.findSecurityGroupRuleByID

	return r, nil
}

type resourceSecurityGroupEgressRule struct {
	resourceSecurityGroupRule
}

// Metadata should return the full name of the resource, such as
// examplecloud_thing.
func (r *resourceSecurityGroupEgressRule) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "aws_vpc_security_group_egress_rule"
}

func (r *resourceSecurityGroupEgressRule) createSecurityGroupRule(ctx context.Context, data *resourceSecurityGroupRuleData) (string, error) {
	conn := r.Meta().EC2Conn

	input := &ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:       aws.String(data.SecurityGroupID.Value),
		IpPermissions: []*ec2.IpPermission{r.expandIPPermission(ctx, data)},
	}

	output, err := conn.AuthorizeSecurityGroupEgressWithContext(ctx, input)

	if err != nil {
		return "", err
	}

	return aws.StringValue(output.SecurityGroupRules[0].SecurityGroupRuleId), nil
}

func (r *resourceSecurityGroupEgressRule) deleteSecurityGroupRule(ctx context.Context, data *resourceSecurityGroupRuleData) error {
	conn := r.Meta().EC2Conn

	_, err := conn.RevokeSecurityGroupEgressWithContext(ctx, &ec2.RevokeSecurityGroupEgressInput{
		GroupId:              aws.String(data.SecurityGroupID.Value),
		SecurityGroupRuleIds: aws.StringSlice([]string{data.ID.Value}),
	})

	return err
}

func (r *resourceSecurityGroupEgressRule) findSecurityGroupRuleByID(ctx context.Context, id string) (*ec2.SecurityGroupRule, error) {
	conn := r.Meta().EC2Conn

	return FindSecurityGroupEgressRuleByID(ctx, conn, id)
}
