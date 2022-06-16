package hashicups

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type resourceWebappBindingType struct{}

type resourceWebappBinding struct {
	p provider
}

// Order Resource schema
func (r resourceWebappBindingType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"name": { // Containe the name of the Binding resource
				Type:     types.StringType,
				Required: true,
			},
			"agw_name": {
				Type:     types.StringType,
				Required: true,
			},
			"agw_rg": {
				Type:     types.StringType,
				Required: true,
			},			
			"backend_address_pool": {
				Required: true,
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"name": {
						Type:     types.StringType,
						Required: true,
					},
					"fqdns": {
						Type: types.ListType{
							ElemType: types.StringType,
						},
						Required: true,
					},
					"ip_addresses": {
						Type: types.ListType{
							ElemType: types.StringType,
						},
						Optional: true,
					},
				}),
			}, 
		},
	}, nil
}

// New resource instance
func (r resourceWebappBindingType) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	return resourceWebappBinding{
		p: *(p.(*provider)),
	}, nil
}

// Create a new resource
func (r resourceWebappBinding) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan WebappBinding
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get the agw

	//Verify if the agw already contains the wanted element

	//create the new agw object

	//update agw 

	//state 

	// Generate API request body from plan
	var backend Backend_address_pool
	backend = plan.Backend_address_pool
	fmt.Printf("#########################################\n")
	fmt.Printf("backend.Name.Value %s\n", backend.Name.Value)

	// Create new order
	
	//command := "script.ps1"// -Backendpool default-citeo-plus-be-pool"
	//out, err := exec.Command("pwsh", "-File",command,"-Backendpool",backend.Name.Value).Output()
	fmt.Printf("##################Executing pwsh script#######################\n")

	command := "dir"
	out, err := exec.Command(command).CombinedOutput()
	fmt.Printf("\n****************************Read out %s\n", out)

	// if there is an error with our execution handle it here
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	var backend_ Backend_address_pool
	err = json.Unmarshal(out, &backend_)
	if err != nil {
		log.Fatalf("=======================json.Unmarshal failed with %s\n", err)
		return
	}
	webappBinding_name := plan.Name
	// Map response body to resource schema attribute
	//and
	// Generate resource state struct
	var result = WebappBinding{
		Name:                 webappBinding_name,
		Backend_address_pool: backend_,
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information
func (r resourceWebappBinding) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	// Get current state
	var state WebappBinding
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get order from API and then update what is in state from what the API returns
	webappBindingName := state.Name.Value

	// Get order current value
	//command := ".\\script.ps1 -Backendpool " + state.Backend_address_pool.Name.Value
	//out, err := exec.Command("powershell", "-NoProfile", command).CombinedOutput()
	//comande := "powershell.exe ./script.ps1 -Backendpool " + state.Backend_address_pool.Name.Value
	//out, err := exec.Command(comande).Output()
	
	command := "script.ps1"
	out, err := exec.Command("pwsh", "-File",command,"-Backendpool",state.Backend_address_pool.Name.Value).CombinedOutput()
	//command := "dir"
	//out, err := exec.Command(command).CombinedOutput()
	fmt.Printf("\n****************************Read out %s\n", out)

	// if there is an error with our execution handle it here
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading webappBinding",
			"Could not read webappBindingName "+webappBindingName+": "+err.Error(),
		)
		return
	}
	var backend_ Backend_address_pool
	err = json.Unmarshal(out, &backend_)
	if err != nil {
		log.Fatalf("json.Unmarshal failed with %s\n", err)
		return
	}

	webappBinding_name := state.Name
	// Map response body to resource schema attribute
	var result = WebappBinding{
		Name:                 webappBinding_name,
		Backend_address_pool: backend_,
	}

	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

// Update resource
func (r resourceWebappBinding) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
}

// Delete resource
func (r resourceWebappBinding) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
}

// Import resource
func (r resourceWebappBinding) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	// Save the import identifier in the id attribute
	//tfsdk.ResourceImportStatePassthroughID(ctx, tftypes.NewAttributePath().WithAttributeName("id"), req, resp)
}
