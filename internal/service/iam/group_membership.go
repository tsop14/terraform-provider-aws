package iam

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func ResourceGroupMembership() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceGroupMembershipCreate,
		ReadWithoutTimeout:   resourceGroupMembershipRead,
		UpdateWithoutTimeout: resourceGroupMembershipUpdate,
		DeleteWithoutTimeout: resourceGroupMembershipDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"users": {
				Type:     schema.TypeSet,
				Required: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"group": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceGroupMembershipCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMConn()

	group := d.Get("group").(string)
	userList := flex.ExpandStringValueSet(d.Get("users").(*schema.Set))

	if err := addUsersToGroup(ctx, conn, userList, group); err != nil {
		return sdkdiag.AppendErrorf(diags, "creating IAM Group Membership (%s): %s", d.Get("name").(string), err)
	}

	d.SetId(d.Get("name").(string))

	return append(diags, resourceGroupMembershipRead(ctx, d, meta)...)
}

func resourceGroupMembershipRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMConn()
	group := d.Get("group").(string)

	input := &iam.GetGroupInput{
		GroupName: aws.String(group),
	}

	var ul []string

	err := resource.RetryContext(ctx, propagationTimeout, func() *resource.RetryError {
		err := conn.GetGroupPagesWithContext(ctx, input, func(page *iam.GetGroupOutput, lastPage bool) bool {
			if page == nil {
				return !lastPage
			}

			for _, user := range page.Users {
				ul = append(ul, aws.StringValue(user.UserName))
			}

			return !lastPage
		})

		if d.IsNewResource() && tfawserr.ErrCodeEquals(err, iam.ErrCodeNoSuchEntityException) {
			return resource.RetryableError(err)
		}

		if err != nil {
			return resource.NonRetryableError(err)
		}

		return nil
	})

	if tfresource.TimedOut(err) {
		err = conn.GetGroupPagesWithContext(ctx, input, func(page *iam.GetGroupOutput, lastPage bool) bool {
			if page == nil {
				return !lastPage
			}

			for _, user := range page.Users {
				ul = append(ul, aws.StringValue(user.UserName))
			}

			return !lastPage
		})
	}

	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, iam.ErrCodeNoSuchEntityException) {
		log.Printf("[WARN] IAM Group Membership (%s) not found, removing from state", group)
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading IAM Group Membership (%s): %s", group, err)
	}

	if err := d.Set("users", ul); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting user list from IAM Group Membership (%s): %s", group, err)
	}

	return diags
}

func resourceGroupMembershipUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMConn()

	if d.HasChange("users") {
		group := d.Get("group").(string)

		o, n := d.GetChange("users")
		if o == nil {
			o = new(schema.Set)
		}
		if n == nil {
			n = new(schema.Set)
		}

		os := o.(*schema.Set)
		ns := n.(*schema.Set)
		remove := flex.ExpandStringValueSet(os.Difference(ns))
		add := flex.ExpandStringValueSet(ns.Difference(os))

		if err := removeUsersFromGroup(ctx, conn, remove, group); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating IAM Group Membership (%s): %s", d.Get("name").(string), err)
		}

		if err := addUsersToGroup(ctx, conn, add, group); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating IAM Group Membership (%s): %s", d.Get("name").(string), err)
		}
	}

	return append(diags, resourceGroupMembershipRead(ctx, d, meta)...)
}

func resourceGroupMembershipDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).IAMConn()
	userList := flex.ExpandStringValueSet(d.Get("users").(*schema.Set))
	group := d.Get("group").(string)

	if err := removeUsersFromGroup(ctx, conn, userList, group); err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting IAM Group Membership (%s): %s", d.Get("name").(string), err)
	}
	return diags
}

func removeUsersFromGroup(ctx context.Context, conn *iam.IAM, users []string, group string) error {
	for _, user := range users {
		if err := removeUserFromGroup(ctx, conn, user, group); err != nil {
			return err
		}
	}
	return nil
}

func addUsersToGroup(ctx context.Context, conn *iam.IAM, users []string, group string) error {
	for _, user := range users {
		if err := addUserToGroup(ctx, conn, user, group); err != nil {
			return err
		}
	}
	return nil
}
