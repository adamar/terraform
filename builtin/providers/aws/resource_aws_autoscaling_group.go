package aws

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/mitchellh/goamz/autoscaling"
)

func resourceAwsAutoscalingGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsAutoscalingGroupCreate,
		Read:   resourceAwsAutoscalingGroupRead,
		Update: resourceAwsAutoscalingGroupUpdate,
		Delete: resourceAwsAutoscalingGroupDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"launch_configuration": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"desired_capacity": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},

			"min_size": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},

			"max_size": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},

			"default_cooldown": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"force_delete": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"health_check_grace_period": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"health_check_type": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"availability_zones": &schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set: func(v interface{}) int {
					return hashcode.String(v.(string))
				},
			},

			"load_balancers": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set: func(v interface{}) int {
					return hashcode.String(v.(string))
				},
			},

			"vpc_zone_identifier": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set: func(v interface{}) int {
					return hashcode.String(v.(string))
				},
			},
		},
	}
}

func resourceAwsAutoscalingGroupCreate(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*ResourceProvider)
	autoscalingconn := p.autoscalingconn

	var autoScalingGroupOpts autoscaling.CreateAutoScalingGroup
	autoScalingGroupOpts.Name = d.Get("name").(string)
	autoScalingGroupOpts.HealthCheckType = d.Get("health_check_type").(string)
	autoScalingGroupOpts.LaunchConfigurationName = d.Get("launch_configuration").(string)
	autoScalingGroupOpts.MinSize = d.Get("min_size").(int)
	autoScalingGroupOpts.MaxSize = d.Get("max_size").(int)
	autoScalingGroupOpts.SetMinSize = true
	autoScalingGroupOpts.SetMaxSize = true
	autoScalingGroupOpts.AvailZone = expandStringList(
		d.Get("availability_zones").(*schema.Set).List())

	if v, ok := d.GetOk("default_cooldown"); ok {
		autoScalingGroupOpts.DefaultCooldown = v.(int)
		autoScalingGroupOpts.SetDefaultCooldown = true
	}

	if v, ok := d.GetOk("desired_capacity"); ok {
		autoScalingGroupOpts.DesiredCapacity = v.(int)
		autoScalingGroupOpts.SetDesiredCapacity = true
	}

	if v, ok := d.GetOk("health_check_grace_period"); ok {
		autoScalingGroupOpts.HealthCheckGracePeriod = v.(int)
		autoScalingGroupOpts.SetHealthCheckGracePeriod = true
	}

	if v, ok := d.GetOk("load_balancers"); ok {
		autoScalingGroupOpts.LoadBalancerNames = expandStringList(
			v.(*schema.Set).List())
	}

	if v, ok := d.GetOk("vpc_zone_identifier"); ok {
		autoScalingGroupOpts.VPCZoneIdentifier = expandStringList(
			v.(*schema.Set).List())
	}

	log.Printf("[DEBUG] AutoScaling Group create configuration: %#v", autoScalingGroupOpts)
	_, err := autoscalingconn.CreateAutoScalingGroup(&autoScalingGroupOpts)
	if err != nil {
		return fmt.Errorf("Error creating Autoscaling Group: %s", err)
	}

	d.SetId(d.Get("name").(string))
	log.Printf("[INFO] AutoScaling Group ID: %s", d.Id())

	return resourceAwsAutoscalingGroupRead(d, meta)
}

func resourceAwsAutoscalingGroupUpdate(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*ResourceProvider)
	autoscalingconn := p.autoscalingconn

	opts := autoscaling.UpdateAutoScalingGroup{
		Name: d.Id(),
	}

	if d.HasChange("desired_capacity") {
		opts.DesiredCapacity = d.Get("desired_capacity").(int)
		opts.SetDesiredCapacity = true
	}

	if d.HasChange("min_size") {
		opts.MinSize = d.Get("min_size").(int)
		opts.SetMinSize = true
	}

	if d.HasChange("max_size") {
		opts.MaxSize = d.Get("max_size").(int)
		opts.SetMaxSize = true
	}

	log.Printf("[DEBUG] AutoScaling Group update configuration: %#v", opts)
	_, err := autoscalingconn.UpdateAutoScalingGroup(&opts)
	if err != nil {
		d.Partial(true)
		return fmt.Errorf("Error updating Autoscaling group: %s", err)
	}

	return resourceAwsAutoscalingGroupRead(d, meta)
}

func resourceAwsAutoscalingGroupDelete(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*ResourceProvider)
	autoscalingconn := p.autoscalingconn

	log.Printf("[DEBUG] AutoScaling Group destroy: %v", d.Id())
	deleteopts := autoscaling.DeleteAutoScalingGroup{Name: d.Id()}

	// You can force an autoscaling group to delete
	// even if it's in the process of scaling a resource.
	// Normally, you would set the min-size and max-size to 0,0
	// and then delete the group. This bypasses that and leaves
	// resources potentially dangling.
	if d.Get("force_delete").(bool) {
		deleteopts.ForceDelete = true
	}

	_, err := autoscalingconn.DeleteAutoScalingGroup(&deleteopts)
	if err != nil {
		autoscalingerr, ok := err.(*autoscaling.Error)
		if ok && autoscalingerr.Code == "InvalidGroup.NotFound" {
			return nil
		}

		return err
	}

	return nil
}

func resourceAwsAutoscalingGroupRead(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*ResourceProvider)
	autoscalingconn := p.autoscalingconn

	describeOpts := autoscaling.DescribeAutoScalingGroups{
		Names: []string{d.Id()},
	}

	log.Printf("[DEBUG] AutoScaling Group describe configuration: %#v", describeOpts)
	describeGroups, err := autoscalingconn.DescribeAutoScalingGroups(&describeOpts)
	if err != nil {
		autoscalingerr, ok := err.(*autoscaling.Error)
		if ok && autoscalingerr.Code == "InvalidGroup.NotFound" {
			d.SetId("")
			return nil
		}

		return fmt.Errorf("Error retrieving AutoScaling groups: %s", err)
	}

	// Verify AWS returned our sg
	if len(describeGroups.AutoScalingGroups) != 1 ||
		describeGroups.AutoScalingGroups[0].Name != d.Id() {
		if err != nil {
			return fmt.Errorf("Unable to find AutoScaling group: %#v", describeGroups.AutoScalingGroups)
		}
	}

	g := describeGroups.AutoScalingGroups[0]

	d.Set("availability_zones", flattenAvailabilityZones(g.AvailabilityZones))
	d.Set("default_cooldown", g.DefaultCooldown)
	d.Set("desired_capacity", g.DesiredCapacity)
	d.Set("health_check_grace_period", g.HealthCheckGracePeriod)
	d.Set("health_check_type", g.HealthCheckType)
	d.Set("launch_configuration", g.LaunchConfigurationName)
	d.Set("min_size", g.MinSize)
	d.Set("max_size", g.MaxSize)
	d.Set("name", g.Name)
	d.Set("vpc_zone_identifier", g.VPCZoneIdentifier)

	if len(g.LoadBalancerNames) > 0 && g.LoadBalancerNames[0].LoadBalancerName != "" {
		d.Set("load_balancers", flattenLoadBalancers(g.LoadBalancerNames))
	}

	return nil
}
