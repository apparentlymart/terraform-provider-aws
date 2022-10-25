package route53resolver

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53resolver"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

func ResourceFirewallRuleGroupAssociation() *schema.Resource {
	return &schema.Resource{
		Create: resourceFirewallRuleGroupAssociationCreate,
		Read:   resourceFirewallRuleGroupAssociationRead,
		Update: resourceFirewallRuleGroupAssociationUpdate,
		Delete: resourceFirewallRuleGroupAssociationDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"firewall_rule_group_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"mutation_protection": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.StringInSlice(route53resolver.MutationProtectionStatus_Values(), false),
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validResolverName,
			},
			"priority": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"tags":     tftags.TagsSchema(),
			"tags_all": tftags.TagsSchemaComputed(),
			"vpc_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},

		CustomizeDiff: verify.SetTagsDiff,
	}
}

func resourceFirewallRuleGroupAssociationCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).Route53ResolverConn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(tftags.New(d.Get("tags").(map[string]interface{})))

	name := d.Get("name").(string)
	input := &route53resolver.AssociateFirewallRuleGroupInput{
		CreatorRequestId:    aws.String(resource.PrefixedUniqueId("tf-r53-rslvr-frgassoc-")),
		FirewallRuleGroupId: aws.String(d.Get("firewall_rule_group_id").(string)),
		Name:                aws.String(name),
		Priority:            aws.Int64(int64(d.Get("priority").(int))),
		VpcId:               aws.String(d.Get("vpc_id").(string)),
	}

	if v, ok := d.GetOk("mutation_protection"); ok {
		input.MutationProtection = aws.String(v.(string))
	}

	if len(tags) > 0 {
		input.Tags = Tags(tags.IgnoreAWS())
	}

	log.Printf("[DEBUG] Creating Route 53 Resolver DNS Firewall rule group association: %#v", input)
	output, err := conn.AssociateFirewallRuleGroup(input)

	if err != nil {
		return fmt.Errorf("creating Route53 Resolver Firewall Rule Group Association (%s): %w", name, err)
	}

	d.SetId(aws.StringValue(output.FirewallRuleGroupAssociation.Id))

	if _, err := waitFirewallRuleGroupAssociationCreated(conn, d.Id()); err != nil {
		return fmt.Errorf("waiting for Route53 Resolver Firewall Rule Group Association (%s) create: %w", d.Id(), err)
	}

	return resourceFirewallRuleGroupAssociationRead(d, meta)
}

func resourceFirewallRuleGroupAssociationRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).Route53ResolverConn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*conns.AWSClient).IgnoreTagsConfig

	ruleGroupAssociation, err := FindFirewallRuleGroupAssociationByID(conn, d.Id())

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] Route53 Resolver Firewall Rule Group Association (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("reading Route53 Resolver Firewall Rule Group Association (%s): %w", d.Id(), err)
	}

	arn := aws.StringValue(ruleGroupAssociation.Arn)
	d.Set("arn", arn)
	d.Set("name", ruleGroupAssociation.Name)
	d.Set("firewall_rule_group_id", ruleGroupAssociation.FirewallRuleGroupId)
	d.Set("mutation_protection", ruleGroupAssociation.MutationProtection)
	d.Set("priority", ruleGroupAssociation.Priority)
	d.Set("vpc_id", ruleGroupAssociation.VpcId)

	tags, err := ListTags(conn, arn)

	if err != nil {
		return fmt.Errorf("listing tags for Route53 Resolver Firewall Rule Group Association (%s): %w", arn, err)
	}

	tags = tags.IgnoreAWS().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return fmt.Errorf("error setting tags: %w", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return fmt.Errorf("error setting tags_all: %w", err)
	}

	return nil
}

func resourceFirewallRuleGroupAssociationUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).Route53ResolverConn

	if d.HasChanges("name", "mutation_protection", "priority") {
		input := &route53resolver.UpdateFirewallRuleGroupAssociationInput{
			FirewallRuleGroupAssociationId: aws.String(d.Id()),
			Name:                           aws.String(d.Get("name").(string)),
			Priority:                       aws.Int64(int64(d.Get("priority").(int))),
		}

		if v, ok := d.GetOk("mutation_protection"); ok {
			input.MutationProtection = aws.String(v.(string))
		}

		log.Printf("[DEBUG] Updating Route 53 Resolver DNS Firewall rule group association: %#v", input)
		_, err := conn.UpdateFirewallRuleGroupAssociation(input)
		if err != nil {
			return fmt.Errorf("error creating Route 53 Resolver DNS Firewall rule group association: %w", err)
		}

		if _, err := waitFirewallRuleGroupAssociationUpdated(conn, d.Id()); err != nil {
			return fmt.Errorf("waiting for Route53 Resolver Firewall Rule Group Association (%s) update: %w", d.Id(), err)
		}
	}

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")
		if err := UpdateTags(conn, d.Get("arn").(string), o, n); err != nil {
			return fmt.Errorf("error updating Route53 Resolver DNS Firewall rule group association (%s) tags: %w", d.Get("arn").(string), err)
		}
	}

	return resourceFirewallRuleGroupAssociationRead(d, meta)
}

func resourceFirewallRuleGroupAssociationDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).Route53ResolverConn

	log.Printf("[DEBUG] Deleting Route53 Resolver Firewall Rule Group Association: %s", d.Id())
	_, err := conn.DisassociateFirewallRuleGroup(&route53resolver.DisassociateFirewallRuleGroupInput{
		FirewallRuleGroupAssociationId: aws.String(d.Id()),
	})

	if tfawserr.ErrCodeEquals(err, route53resolver.ErrCodeResourceNotFoundException) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("deleting Route53 Resolver Firewall Rule Group Association (%s): %w", d.Id(), err)
	}

	if _, err := waitFirewallRuleGroupAssociationDeleted(conn, d.Id()); err != nil {
		return fmt.Errorf("waiting for Route53 Resolver Firewall Rule Group Association (%s) delete: %w", d.Id(), err)
	}

	return nil
}

func FindFirewallRuleGroupAssociationByID(conn *route53resolver.Route53Resolver, id string) (*route53resolver.FirewallRuleGroupAssociation, error) {
	input := &route53resolver.GetFirewallRuleGroupAssociationInput{
		FirewallRuleGroupAssociationId: aws.String(id),
	}

	output, err := conn.GetFirewallRuleGroupAssociation(input)

	if tfawserr.ErrCodeEquals(err, route53resolver.ErrCodeResourceNotFoundException) {
		return nil, &resource.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil || output.FirewallRuleGroupAssociation == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	return output.FirewallRuleGroupAssociation, nil
}

func statusFirewallRuleGroupAssociation(conn *route53resolver.Route53Resolver, id string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		output, err := FindFirewallRuleGroupAssociationByID(conn, id)

		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return output, aws.StringValue(output.Status), nil
	}
}

const (
	firewallRuleGroupAssociationCreatedTimeout = 5 * time.Minute
	firewallRuleGroupAssociationUpdatedTimeout = 5 * time.Minute
	firewallRuleGroupAssociationDeletedTimeout = 5 * time.Minute
)

func waitFirewallRuleGroupAssociationCreated(conn *route53resolver.Route53Resolver, id string) (*route53resolver.FirewallRuleGroupAssociation, error) {
	stateConf := &resource.StateChangeConf{
		Pending: []string{route53resolver.FirewallRuleGroupAssociationStatusUpdating},
		Target:  []string{route53resolver.FirewallRuleGroupAssociationStatusComplete},
		Refresh: statusFirewallRuleGroupAssociation(conn, id),
		Timeout: firewallRuleGroupAssociationCreatedTimeout,
	}

	outputRaw, err := stateConf.WaitForState()

	if output, ok := outputRaw.(*route53resolver.FirewallRuleGroupAssociation); ok {
		tfresource.SetLastError(err, errors.New(aws.StringValue(output.StatusMessage)))

		return output, err
	}

	return nil, err
}

func waitFirewallRuleGroupAssociationUpdated(conn *route53resolver.Route53Resolver, id string) (*route53resolver.FirewallRuleGroupAssociation, error) {
	stateConf := &resource.StateChangeConf{
		Pending: []string{route53resolver.FirewallRuleGroupAssociationStatusUpdating},
		Target:  []string{route53resolver.FirewallRuleGroupAssociationStatusComplete},
		Refresh: statusFirewallRuleGroupAssociation(conn, id),
		Timeout: firewallRuleGroupAssociationUpdatedTimeout,
	}

	outputRaw, err := stateConf.WaitForState()

	if output, ok := outputRaw.(*route53resolver.FirewallRuleGroupAssociation); ok {
		tfresource.SetLastError(err, errors.New(aws.StringValue(output.StatusMessage)))

		return output, err
	}

	return nil, err
}

func waitFirewallRuleGroupAssociationDeleted(conn *route53resolver.Route53Resolver, id string) (*route53resolver.FirewallRuleGroupAssociation, error) {
	stateConf := &resource.StateChangeConf{
		Pending: []string{route53resolver.FirewallRuleGroupAssociationStatusDeleting},
		Target:  []string{},
		Refresh: statusFirewallRuleGroupAssociation(conn, id),
		Timeout: firewallRuleGroupAssociationDeletedTimeout,
	}

	outputRaw, err := stateConf.WaitForState()

	if output, ok := outputRaw.(*route53resolver.FirewallRuleGroupAssociation); ok {
		tfresource.SetLastError(err, errors.New(aws.StringValue(output.StatusMessage)))

		return output, err
	}

	return nil, err
}
