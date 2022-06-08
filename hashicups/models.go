package hashicups

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Order -
type Order struct {
	ID          types.String `tfsdk:"id"`
	Items       []OrderItem  `tfsdk:"items"`
	LastUpdated types.String `tfsdk:"last_updated"`
}

// OrderItem -
type OrderItem struct {
	Coffee   Coffee `tfsdk:"coffee"`
	Quantity int    `tfsdk:"quantity"`
}

// Coffee -
// This Coffee struct is for Order.Items[].Coffee which does not have an
// ingredients field in the schema defined by plugin framework. Since the
// resource schema must match the struct exactly (extra field will return an
// error). This struct has Ingredients commented out.
type Coffee struct {
	ID          int          `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Teaser      types.String `tfsdk:"teaser"`
	Description types.String `tfsdk:"description"`
	Price       types.Number `tfsdk:"price"`
	Image       types.String `tfsdk:"image"`
	// Ingredients []Ingredient   `tfsdk:"ingredients"`
}

// WebappBinding -
type WebappBinding struct {
	Name					types.String			`tfsdk:"name"`
	Backend_address_pool    Backend_address_pool 	`tfsdk:"backend_address_pool"`
	//Backend_http_settings   Backend_http_settings	`tfsdk:"backend_http_settings"`
}
type Backend_address_pool struct {
	Name			types.String	`tfsdk:"name"`
	Fqdns   		[]types.String	`tfsdk:"fqdns"`	
	Ip_addresses	[]types.String	`tfsdk:"ip_addresses"`
}/*
type Backend_http_settings struct {
	Name		types.String	`tfsdk:"name"`
	Protocol	types.String	`tfsdk:"protocol"`	
}*/
