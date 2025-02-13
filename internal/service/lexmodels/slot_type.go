package lexmodels

import (
	"context"
	"log"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lexmodelbuildingservice"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

const (
	slotTypeCreateTimeout = 1 * time.Minute
	slotTypeUpdateTimeout = 1 * time.Minute
	slotTypeDeleteTimeout = 5 * time.Minute
)

func ResourceSlotType() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceSlotTypeCreate,
		ReadWithoutTimeout:   resourceSlotTypeRead,
		UpdateWithoutTimeout: resourceSlotTypeUpdate,
		DeleteWithoutTimeout: resourceSlotTypeDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(slotTypeCreateTimeout),
			Update: schema.DefaultTimeout(slotTypeUpdateTimeout),
			Delete: schema.DefaultTimeout(slotTypeDeleteTimeout),
		},

		Schema: map[string]*schema.Schema{
			"checksum": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"create_version": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"created_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"description": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "",
				ValidateFunc: validation.StringLenBetween(0, 200),
			},
			"enumeration_value": {
				Type:     schema.TypeSet,
				Required: true,
				MinItems: 1,
				MaxItems: 10000,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"synonyms": {
							Type:     schema.TypeSet,
							Optional: true,
							MinItems: 1,
							Elem: &schema.Schema{
								Type:         schema.TypeString,
								ValidateFunc: validation.StringLenBetween(1, 140),
							},
						},
						"value": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringLenBetween(1, 140),
						},
					},
				},
			},
			"last_updated_date": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(1, 100),
					validation.StringMatch(regexp.MustCompile(`^((AMAZON\.)_?|[A-Za-z]_?)+`), ""),
				),
			},
			"value_selection_strategy": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      lexmodelbuildingservice.SlotValueSelectionStrategyOriginalValue,
				ValidateFunc: validation.StringInSlice(lexmodelbuildingservice.SlotValueSelectionStrategy_Values(), false),
			},
			"version": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
		CustomizeDiff: updateComputedAttributesOnSlotTypeCreateVersion,
	}
}

func updateComputedAttributesOnSlotTypeCreateVersion(_ context.Context, d *schema.ResourceDiff, meta interface{}) error {
	createVersion := d.Get("create_version").(bool)
	if createVersion && hasSlotTypeConfigChanges(d) {
		d.SetNewComputed("version")
	}
	return nil
}

func hasSlotTypeConfigChanges(d verify.ResourceDiffer) bool {
	for _, key := range []string{
		"description",
		"enumeration_value",
		"value_selection_strategy",
	} {
		if d.HasChange(key) {
			return true
		}
	}
	return false
}

func resourceSlotTypeCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).LexModelsConn()

	name := d.Get("name").(string)
	input := &lexmodelbuildingservice.PutSlotTypeInput{
		CreateVersion:          aws.Bool(d.Get("create_version").(bool)),
		Description:            aws.String(d.Get("description").(string)),
		Name:                   aws.String(name),
		ValueSelectionStrategy: aws.String(d.Get("value_selection_strategy").(string)),
	}

	if v, ok := d.GetOk("enumeration_value"); ok {
		input.EnumerationValues = expandEnumerationValues(v.(*schema.Set).List())
	}

	var output *lexmodelbuildingservice.PutSlotTypeOutput
	_, err := tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutCreate), func() (interface{}, error) {
		var err error

		if output != nil {
			input.Checksum = output.Checksum
		}
		output, err = conn.PutSlotTypeWithContext(ctx, input)

		return output, err
	}, lexmodelbuildingservice.ErrCodeConflictException)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating Lex Slot Type (%s): %s", name, err)
	}

	d.SetId(name)

	return append(diags, resourceSlotTypeRead(ctx, d, meta)...)
}

func resourceSlotTypeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).LexModelsConn()

	output, err := FindSlotTypeVersionByName(ctx, conn, d.Id(), SlotTypeVersionLatest)

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] Lex Slot Type (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading Lex Slot Type (%s): %s", d.Id(), err)
	}

	d.Set("checksum", output.Checksum)
	d.Set("created_date", output.CreatedDate.Format(time.RFC3339))
	d.Set("description", output.Description)
	d.Set("last_updated_date", output.LastUpdatedDate.Format(time.RFC3339))
	d.Set("name", output.Name)
	d.Set("value_selection_strategy", output.ValueSelectionStrategy)

	if output.EnumerationValues != nil {
		d.Set("enumeration_value", flattenEnumerationValues(output.EnumerationValues))
	}

	version, err := FindLatestSlotTypeVersionByName(ctx, conn, d.Id())

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading Lex Slot Type (%s) latest version: %s", d.Id(), err)
	}

	d.Set("version", version)

	return diags
}

func resourceSlotTypeUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).LexModelsConn()

	input := &lexmodelbuildingservice.PutSlotTypeInput{
		Checksum:               aws.String(d.Get("checksum").(string)),
		CreateVersion:          aws.Bool(d.Get("create_version").(bool)),
		Description:            aws.String(d.Get("description").(string)),
		Name:                   aws.String(d.Id()),
		ValueSelectionStrategy: aws.String(d.Get("value_selection_strategy").(string)),
	}

	if v, ok := d.GetOk("enumeration_value"); ok {
		input.EnumerationValues = expandEnumerationValues(v.(*schema.Set).List())
	}

	_, err := tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutUpdate), func() (interface{}, error) {
		return conn.PutSlotTypeWithContext(ctx, input)
	}, lexmodelbuildingservice.ErrCodeConflictException)

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "updating Lex Slot Type (%s): %s", d.Id(), err)
	}

	return append(diags, resourceSlotTypeRead(ctx, d, meta)...)
}

func resourceSlotTypeDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).LexModelsConn()

	input := &lexmodelbuildingservice.DeleteSlotTypeInput{
		Name: aws.String(d.Id()),
	}

	log.Printf("[DEBUG] Deleting Lex Slot Type: (%s)", d.Id())
	_, err := tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutDelete), func() (interface{}, error) {
		return conn.DeleteSlotTypeWithContext(ctx, input)
	}, lexmodelbuildingservice.ErrCodeConflictException)

	if tfawserr.ErrCodeEquals(err, lexmodelbuildingservice.ErrCodeNotFoundException) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting Lex Model Slot Type (%s): %s", d.Id(), err)
	}

	if _, err := waitSlotTypeDeleted(ctx, conn, d.Id()); err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting Lex Model Slot Type (%s): waiting for completion: %s", d.Id(), err)
	}

	return diags
}

func flattenEnumerationValues(values []*lexmodelbuildingservice.EnumerationValue) (flattened []map[string]interface{}) {
	for _, value := range values {
		flattened = append(flattened, map[string]interface{}{
			"synonyms": flex.FlattenStringList(value.Synonyms),
			"value":    aws.StringValue(value.Value),
		})
	}

	return
}

func expandEnumerationValues(rawValues []interface{}) []*lexmodelbuildingservice.EnumerationValue {
	enums := make([]*lexmodelbuildingservice.EnumerationValue, 0, len(rawValues))
	for _, rawValue := range rawValues {
		value, ok := rawValue.(map[string]interface{})
		if !ok {
			continue
		}

		enums = append(enums, &lexmodelbuildingservice.EnumerationValue{
			Synonyms: flex.ExpandStringSet(value["synonyms"].(*schema.Set)),
			Value:    aws.String(value["value"].(string)),
		})
	}
	return enums
}
