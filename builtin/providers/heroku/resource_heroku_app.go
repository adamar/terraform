package heroku

import (
	"fmt"
	"log"

	"github.com/cyberdelia/heroku-go/v3"
	"github.com/hashicorp/terraform/helper/multierror"
	"github.com/hashicorp/terraform/helper/schema"
)

// type application is used to store all the details of a heroku app
type application struct {
	Id string // Id of the resource

	App    *heroku.App       // The heroku application
	Client *heroku.Service   // Client to interact with the heroku API
	Vars   map[string]string // The vars on the application
}

// Updates the application to have the latest from remote
func (a *application) Update() error {
	var errs []error
	var err error

	a.App, err = a.Client.AppInfo(a.Id)
	if err != nil {
		errs = append(errs, err)
	}

	a.Vars, err = retrieve_config_vars(a.Id, a.Client)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return &multierror.Error{Errors: errs}
	}

	return nil
}

func resourceHerokuApp() *schema.Resource {
	return &schema.Resource{
		Create: switchHerokuAppCreate,
		Read:   resourceHerokuAppRead,
		Update: resourceHerokuAppUpdate,
		Delete: resourceHerokuAppDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},

			"region": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"stack": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"config_vars": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
				},
			},

			"all_config_vars": &schema.Schema{
				Type:     schema.TypeMap,
				Computed: true,
			},

			"git_url": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"web_url": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"heroku_hostname": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"organization": &schema.Schema{
				Type:        schema.TypeString,
				Description: "Name of Organization to create application in. Leave blank for personal apps.",
				Optional:    true,
				ForceNew:    true,
			},
		},
	}
}

func switchHerokuAppCreate(d *schema.ResourceData, meta interface{}) error {
	if _, ok := d.GetOk("organization"); ok {
		return resourceHerokuOrgAppCreate(d, meta)
	} else {
		return resourceHerokuAppCreate(d, meta)
	}
}

func resourceHerokuAppCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*heroku.Service)

	// Build up our creation options
	opts := heroku.AppCreateOpts{}

	if v, ok := d.GetOk("name"); ok {
		vs := v.(string)
		log.Printf("[DEBUG] App name: %s", vs)
		opts.Name = &vs
	}
	if v, ok := d.GetOk("region"); ok {
		vs := v.(string)
		log.Printf("[DEBUG] App region: %s", vs)
		opts.Region = &vs
	}
	if v, ok := d.GetOk("stack"); ok {
		vs := v.(string)
		log.Printf("[DEBUG] App stack: %s", vs)
		opts.Stack = &vs
	}

	log.Printf("[DEBUG] Creating Heroku app...")
	a, err := client.AppCreate(opts)
	if err != nil {
		return err
	}

	d.SetId(a.Name)
	log.Printf("[INFO] App ID: %s", d.Id())

	if v, ok := d.GetOk("config_vars"); ok {
		err = update_config_vars(d.Id(), client, nil, v.([]interface{}))
		if err != nil {
			return err
		}
	}

	return resourceHerokuAppRead(d, meta)
}

func resourceHerokuOrgAppCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*heroku.Service)
	// Build up our creation options
	opts := heroku.OrganizationAppCreateOpts{}
	if v, ok := d.GetOk("organization"); ok {
		vs := v.(string)
		log.Printf("[DEBUG] App name: %s", vs)
		opts.Organization = &vs
	}
	if v, ok := d.GetOk("name"); ok {
		vs := v.(string)
		log.Printf("[DEBUG] App name: %s", vs)
		opts.Name = &vs
	}
	if v, ok := d.GetOk("region"); ok {
		vs := v.(string)
		log.Printf("[DEBUG] App region: %s", vs)
		opts.Region = &vs
	}
	if v, ok := d.GetOk("stack"); ok {
		vs := v.(string)
		log.Printf("[DEBUG] App stack: %s", vs)
		opts.Stack = &vs
	}

	log.Printf("[DEBUG] Creating Heroku app...")
	a, err := client.OrganizationAppCreate(opts)
	if err != nil {
		return err
	}

	d.SetId(a.Name)
	log.Printf("[INFO] App ID: %s", d.Id())

	if v, ok := d.GetOk("config_vars"); ok {
		err = update_config_vars(d.Id(), client, nil, v.([]interface{}))
		if err != nil {
			return err
		}
	}

	return resourceHerokuAppRead(d, meta)
}

func resourceHerokuAppRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*heroku.Service)
	app, err := resource_heroku_app_retrieve(d.Id(), client)
	if err != nil {
		return err
	}

	// Only set the config_vars that we have set in the configuration.
	// The "all_config_vars" field has all of them.
	configVars := make(map[string]string)
	care := make(map[string]struct{})
	for _, v := range d.Get("config_vars").([]interface{}) {
		for k, _ := range v.(map[string]interface{}) {
			care[k] = struct{}{}
		}
	}
	for k, v := range app.Vars {
		if _, ok := care[k]; ok {
			configVars[k] = v
		}
	}
	var configVarsValue []map[string]string
	if len(configVars) > 0 {
		configVarsValue = []map[string]string{configVars}
	}

	d.Set("name", app.App.Name)
	d.Set("stack", app.App.Stack.Name)
	d.Set("region", app.App.Region.Name)
	d.Set("git_url", app.App.GitURL)
	d.Set("web_url", app.App.WebURL)
	d.Set("config_vars", configVarsValue)
	d.Set("all_config_vars", app.Vars)

	// We know that the hostname on heroku will be the name+herokuapp.com
	// You need this to do things like create DNS CNAME records
	d.Set("heroku_hostname", fmt.Sprintf("%s.herokuapp.com", app.App.Name))

	return nil
}

func resourceHerokuAppUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*heroku.Service)

	// If name changed, update it
	if d.HasChange("name") {
		v := d.Get("name").(string)
		opts := heroku.AppUpdateOpts{
			Name: &v,
		}

		renamedApp, err := client.AppUpdate(d.Id(), opts)
		if err != nil {
			return err
		}

		// Store the new ID
		d.SetId(renamedApp.Name)
	}

	// If the config vars changed, then recalculate those
	if d.HasChange("config_vars") {
		o, n := d.GetChange("config_vars")
		if o == nil {
			o = []interface{}{}
		}
		if n == nil {
			n = []interface{}{}
		}

		err := update_config_vars(
			d.Id(), client, o.([]interface{}), n.([]interface{}))
		if err != nil {
			return err
		}
	}

	return resourceHerokuAppRead(d, meta)
}

func resourceHerokuAppDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*heroku.Service)

	log.Printf("[INFO] Deleting App: %s", d.Id())
	err := client.AppDelete(d.Id())
	if err != nil {
		return fmt.Errorf("Error deleting App: %s", err)
	}

	d.SetId("")
	return nil
}

func resource_heroku_app_retrieve(id string, client *heroku.Service) (*application, error) {
	app := application{Id: id, Client: client}

	err := app.Update()

	if err != nil {
		return nil, fmt.Errorf("Error retrieving app: %s", err)
	}

	return &app, nil
}

func retrieve_config_vars(id string, client *heroku.Service) (map[string]string, error) {
	vars, err := client.ConfigVarInfo(id)

	if err != nil {
		return nil, err
	}

	return vars, nil
}

// Updates the config vars for from an expanded configuration.
func update_config_vars(
	id string,
	client *heroku.Service,
	o []interface{},
	n []interface{}) error {
	vars := make(map[string]*string)

	for _, v := range o {
		for k, _ := range v.(map[string]interface{}) {
			vars[k] = nil
		}
	}
	for _, v := range n {
		for k, v := range v.(map[string]interface{}) {
			val := v.(string)
			vars[k] = &val
		}
	}

	log.Printf("[INFO] Updating config vars: *%#v", vars)
	if _, err := client.ConfigVarUpdate(id, vars); err != nil {
		return fmt.Errorf("Error updating config vars: %s", err)
	}

	return nil
}
