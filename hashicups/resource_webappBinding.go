package hashicups

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"terraform-provider-hashicups-pf/azureagw"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
					"id": {
						Type:     types.StringType,
						Computed: true,
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
	resourceGroupName := plan.Agw_rg.Value
	applicationGatewayName := plan.Agw_name.Value
	gw := getGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, r.p.token.Access_token)

	//Verify if the agw already contains the wanted element
	var backend_plan Backend_address_pool
	backend_plan = plan.Backend_address_pool
	resp.Diagnostics.AddWarning("################ Backend Address Pool Name: ", backend_plan.Name.Value)
	if checkBackendAddressPoolElement(gw, backend_plan.Name.Value) {
		// Error  - existing backend_plan address pool name must stop execution
		resp.Diagnostics.AddError(
			"Unable to create Backend Address pool",
			"Backend Address pool Name already exists in the app gateway",
		)
		return
	}

	//create and map the new backend_json object from the backend_plan
	backend_json := azureagw.BackendAddressPools{
		Name: backend_plan.Name.Value,
		Properties: struct {
			ProvisioningState string "json:\"provisioningState,omitempty\""
			BackendAddresses  []struct {
				Fqdn      string "json:\"fqdn,omitempty\""
				IPAddress string "json:\"ipAddress,omitempty\""
			} "json:\"backendAddresses\""
			RequestRoutingRules []struct {
				ID string "json:\"id\""
			} "json:\"requestRoutingRules,omitempty\""
		}{},
		Type: "Microsoft.Network/applicationGateways/backendAddressPools",
	}

	backend_json.Properties.BackendAddresses = make([]struct {
		Fqdn      string "json:\"fqdn,omitempty\""
		IPAddress string "json:\"ipAddress,omitempty\""
	}, 2)
	backend_json.Properties.BackendAddresses[0].Fqdn = backend_plan.Fqdns[0].Value
	backend_json.Properties.BackendAddresses[1].IPAddress = backend_plan.Ip_addresses[0].Value

	// add the backend to the agw and update the agw
	gw.Properties.BackendAddressPools = append(gw.Properties.BackendAddressPools, backend_json)
	gw_response, responseData := updateGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, gw, r.p.token.Access_token)

	rs := string(responseData)
	ress_error, err := PrettyString(rs)
	if err != nil {
		log.Fatal(err)
	}

	args := "\nAZURE_SUBSCRIPTION_ID = " + r.p.AZURE_SUBSCRIPTION_ID +
		"\nresourceGroupName = " + resourceGroupName +
		"\napplicationGatewayName = " + applicationGatewayName +
		"\nAccess_token = " + r.p.token.Access_token
	//tranform the gw json to readible pretty string
	ress_gw := PrettyStringGW(gw_response)

	//verify if the backend address pool is added to the gateway
	if !checkBackendAddressPoolElement(gw_response, backend_json.Name) {
		// Error  - backend address pool wasn't added to the app gateway
		resp.Diagnostics.AddError(
			"Unable to create Backend Address pool ######## API response = "+args+"\n"+ress_error, //+ress_gw+"\n"  
			"Backend Address pool Name doesn't exist in the response app gateway",
		)
		return
	}
	resp.Diagnostics.AddWarning(
		"Unable to create Backend Address pool ######## API response = "+ress_gw+"\n",
		"Backend Address pool Name doesn't exist in the response app gateway",
	)
	// log the added backend address pool
	i := getBackendAddressPoolElementKey(gw_response, backend_json.Name)
	tflog.Trace(ctx, "created BackendAddressPool", "BackendAddressPool ID", gw_response.Properties.BackendAddressPools[i].ID)

	// Map response body to resource schema attribute
	var backend_response Backend_address_pool
	backend_response.Name = types.String{Value: gw_response.Properties.BackendAddressPools[i].Name}
	backend_response.Id = types.String{Value: gw_response.Properties.BackendAddressPools[i].ID}
	backend_response.Fqdns[0] = types.String{Value: gw_response.Properties.BackendAddressPools[i].Properties.BackendAddresses[0].Fqdn}
	backend_response.Ip_addresses[0] = types.String{Value: gw_response.Properties.BackendAddressPools[i].Properties.BackendAddresses[1].IPAddress}

	//and
	// Generate resource state struct
	var result = WebappBinding{
		Name:                 plan.Name,
		Agw_name:             types.String{Value: gw_response.Name},
		Agw_rg:               plan.Agw_rg,
		Backend_address_pool: backend_response,
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
	out, err := exec.Command("pwsh", "-File", command, "-Backendpool", state.Backend_address_pool.Name.Value).CombinedOutput()
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

//Client operations
func getGW(subscriptionId string, resourceGroupName string, applicationGatewayName string, token string) azureagw.ApplicationGateway {
	requestURI := "https://management.azure.com/subscriptions/" + subscriptionId + "/resourceGroups/" +
		resourceGroupName + "/providers/Microsoft.Network/applicationGateways/" + applicationGatewayName + "?api-version=2021-08-01"
	req, err := http.NewRequest("GET", requestURI, nil)
	if err != nil {
		// handle err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Call failure: %+v", err)
	}
	defer resp.Body.Close()
	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	/*
			responseString := string(responseData)
		    fmt.Printf(responseString)


			res, err := PrettyString(responseString)
		    if err != nil {
		        log.Fatal(err)
		    }*/
	//fmt.Println(res)
	var agw azureagw.ApplicationGateway
	err = json.Unmarshal(responseData, &agw)

	if err != nil {
		fmt.Println(err)
	}

	return agw
}
func updateGW(subscriptionId string, resourceGroupName string, applicationGatewayName string, gw azureagw.ApplicationGateway, token string) (azureagw.ApplicationGateway, []byte) {
	requestURI := "https://management.azure.com/subscriptions/" + subscriptionId + "/resourceGroups/" +
		resourceGroupName + "/providers/Microsoft.Network/applicationGateways/" + applicationGatewayName + "?api-version=2021-08-01"
	payloadBytes, err := json.Marshal(gw)
	if err != nil {
		log.Fatal(err)
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("PUT", requestURI, body)
	if err != nil {
		// handle err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Call failure: %+v", err)
	}
	defer resp.Body.Close()

	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var agw azureagw.ApplicationGateway
	err = json.Unmarshal(responseData, &agw)

	if err != nil {
		fmt.Println(err)
	}
	return agw, responseData
}

//Application gateway manipulation
func checkBackendAddressPoolElement(gw azureagw.ApplicationGateway, backendAddressPoolName string) bool {
	exist := false
	for i := len(gw.Properties.BackendAddressPools) - 1; i >= 0; i-- {
		if gw.Properties.BackendAddressPools[i].Name == backendAddressPoolName {
			//gw.Properties.BackendAddressPools =append(gw.Properties.BackendAddressPools[:i], gw.Properties.BackendAddressPools[i+1:]...)
			exist = true
		}
	}
	return exist
}
func removeBackendAddressPoolElement(gw *azureagw.ApplicationGateway, backendAddressPoolName string) {
	removed := false
	for i := len(gw.Properties.BackendAddressPools) - 1; i >= 0; i-- {
		if gw.Properties.BackendAddressPools[i].Name == backendAddressPoolName {
			gw.Properties.BackendAddressPools = append(gw.Properties.BackendAddressPools[:i], gw.Properties.BackendAddressPools[i+1:]...)
			removed = true
		}
	}
	fmt.Println("#############################removed =", removed)
}
func getBackendAddressPoolElementKey(gw azureagw.ApplicationGateway, backendAddressPoolName string) int {
	key := -1
	for i := len(gw.Properties.BackendAddressPools) - 1; i >= 0; i-- {
		if gw.Properties.BackendAddressPools[i].Name == backendAddressPoolName {
			key = i
		}
	}
	return key
}
func PrettyStringGW(gw azureagw.ApplicationGateway) string {
	payloadBytes, err := json.Marshal(gw)
	if err != nil {
		// handle err
	}
	str := string(payloadBytes)
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		return "error"
	}
	return prettyJSON.String()
}
func PrettyString(str string) (string, error) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}
