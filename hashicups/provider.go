package hashicups

import (
	"context"
	"os"

	"github.com/hashicorp-demoapp/hashicups-client-go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var stderr = os.Stderr

func New() tfsdk.Provider {
	return &provider{}
}

type provider struct {
	configured bool
	client     *hashicups.Client
}

// GetSchema
func (p *provider) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"host": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"username": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"password": {
				Type:      types.StringType,
				Optional:  true,
				Computed:  true,
				Sensitive: true,
			},
			"AZURE_CLIENT_ID": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"AZURE_CLIENT_SECRET": {
				Type:     types.StringType,
				Optional:  true,
				Computed:  true,
				Sensitive: true,
			},
			"AZURE_TENANT_ID": {
				Type:     types.StringType,
				Optional:  true,
				Computed:  true,
				Sensitive: true,
			},
		},
	}, nil
}

// Provider schema struct
type providerData struct {
	Username 			types.String `tfsdk:"username"`
	Host     			types.String `tfsdk:"host"`
	Password 			types.String `tfsdk:"password"`
	AZURE_CLIENT_ID 	types.String `tfsdk:"AZURE_CLIENT_ID"`
	AZURE_CLIENT_SECRET types.String `tfsdk:"AZURE_CLIENT_SECRET"`
	AZURE_TENANT_ID 	types.String `tfsdk:"AZURE_TENANT_ID"`
}

func (p *provider) Configure(ctx context.Context, req tfsdk.ConfigureProviderRequest, resp *tfsdk.ConfigureProviderResponse) {
	// Retrieve provider data from configuration
	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// User must provide a user to the provider
	var username string
	if config.Username.Unknown {
		// Cannot connect to client with an unknown value
		resp.Diagnostics.AddWarning(
			"Unable to create client",
			"Cannot use unknown value as username",
		)
		return
	}

	if config.Username.Null {
		username = os.Getenv("HASHICUPS_USERNAME")
	} else {
		username = config.Username.Value
	}

	if username == "" {
		// Error vs warning - empty value must stop execution
		resp.Diagnostics.AddError(
			"Unable to find username",
			"Username cannot be an empty string",
		)
		return
	}

	// User must provide a password to the provider
	var password string
	if config.Password.Unknown {
		// Cannot connect to client with an unknown value
		resp.Diagnostics.AddError(
			"Unable to create client",
			"Cannot use unknown value as password",
		)
		return
	}

	if config.Password.Null {
		password = os.Getenv("HASHICUPS_PASSWORD")
	} else {
		password = config.Password.Value
	}

	if password == "" {
		// Error vs warning - empty value must stop execution
		resp.Diagnostics.AddError(
			"Unable to find password",
			"password cannot be an empty string",
		)
		return
	}

	// User must specify a host
	var host string
	if config.Host.Unknown {
		// Cannot connect to client with an unknown value
		resp.Diagnostics.AddError(
			"Unable to create client",
			"Cannot use unknown value as host",
		)
		return
	}

	if config.Host.Null {
		host = os.Getenv("HASHICUPS_HOST")
	} else {
		host = config.Host.Value
	}

	if host == "" {
		// Error vs warning - empty value must stop execution
		resp.Diagnostics.AddError(
			"Unable to find host",
			"Host cannot be an empty string",
		)
		return
	}

	// User must provide a AZURE_CLIENT_ID to the provider
	var AZURE_CLIENT_ID string
	if config.AZURE_CLIENT_ID.Unknown {
		// Cannot connect to client with an unknown value
		resp.Diagnostics.AddWarning(
			"Unable to create Azure client",
			"Cannot use unknown value as AZURE_CLIENT_ID",
		)
		return
	}

	if config.AZURE_CLIENT_ID.Null {
		AZURE_CLIENT_ID = os.Getenv("AZURE_CLIENT_ID")
	} else {
		AZURE_CLIENT_ID = config.AZURE_CLIENT_ID.Value
	}

	if AZURE_CLIENT_ID == "" {
		// Error vs warning - empty value must stop execution
		resp.Diagnostics.AddError(
			"Unable to find AZURE_CLIENT_ID",
			"AZURE_CLIENT_ID cannot be an empty string",
		)
		return
	}

	// User must provide a AZURE_CLIENT_SECRET to the provider
	var AZURE_CLIENT_SECRET string
	if config.AZURE_CLIENT_SECRET.Unknown {
		// Cannot connect to client with an unknown value
		resp.Diagnostics.AddWarning(
			"Unable to create Azure client",
			"Cannot use unknown value as AZURE_CLIENT_SECRET",
		)
		return
	}

	if config.AZURE_CLIENT_SECRET.Null {
		AZURE_CLIENT_SECRET = os.Getenv("AZURE_CLIENT_SECRET")
	} else {
		AZURE_CLIENT_SECRET = config.AZURE_CLIENT_SECRET.Value
	}

	if AZURE_CLIENT_SECRET == "" {
		// Error vs warning - empty value must stop execution
		resp.Diagnostics.AddError(
			"Unable to find AZURE_CLIENT_SECRET",
			"AZURE_CLIENT_SECRET cannot be an empty string",
		)
		return
	}

	// User must provide a AZURE_TENANT_ID to the provider
	var AZURE_TENANT_ID string
	if config.AZURE_TENANT_ID.Unknown {
		// Cannot connect to client with an unknown value
		resp.Diagnostics.AddWarning(
			"Unable to create Azure client",
			"Cannot use unknown value as AZURE_TENANT_ID",
		)
		return
	}

	if config.AZURE_TENANT_ID.Null {
		AZURE_TENANT_ID = os.Getenv("AZURE_TENANT_ID")
	} else {
		AZURE_TENANT_ID = config.AZURE_TENANT_ID.Value
	}

	if AZURE_TENANT_ID == "" {
		// Error vs warning - empty value must stop execution
		resp.Diagnostics.AddError(
			"Unable to find AZURE_TENANT_ID",
			"AZURE_TENANT_ID cannot be an empty string",
		)
		return
	}


	// Create a new HashiCups client and set it to the provider client
	c, err := hashicups.NewClient(&host, &username, &password)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create client",
			"Unable to create hashicups client:\n\n"+err.Error(),
		)
		return
	}

	p.client = c
	p.configured = true
}

// GetResources - Defines provider resources
func (p *provider) GetResources(_ context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
	return map[string]tfsdk.ResourceType{
		"hashicups_order": resourceOrderType{},
		"hashicups_webappBinding": resourceWebappBindingType{},
	}, nil
}

// GetDataSources - Defines provider data sources
func (p *provider) GetDataSources(_ context.Context) (map[string]tfsdk.DataSourceType, diag.Diagnostics) {
	return map[string]tfsdk.DataSourceType{
		"hashicups_coffees": dataSourceCoffeesType{},
	}, nil
}
