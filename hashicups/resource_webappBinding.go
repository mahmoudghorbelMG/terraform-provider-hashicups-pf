package hashicups

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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
	//resp.Diagnostics.AddWarning("################ Backend Address Pool Name: ", backend_plan.Name.Value)
	if checkBackendAddressPoolElement(gw, backend_plan.Name.Value) {
		// Error  - existing backend_plan address pool name must stop execution
		resp.Diagnostics.AddError(
			"Unable to create Backend Address pool",
			"Backend Address pool Name already exists in the app gateway",
		)
		return
	}

	//create and map the new backend_json object from the backend_plan
	backend_json := azureagw.BackendAddressPool{
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
	gw_response, responseData, code := updateGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, gw, r.p.token.Access_token)

	//if there is an error, responseData contains the error message in jason, else, gw_response is a correct gw Object
	rs := string(responseData)
	ress_error, err := PrettyString(rs)
	if err != nil {
		log.Fatal(err)
	}

	//verify if the backend address pool is added to the gateway
	if !checkBackendAddressPoolElement(gw_response, backend_json.Name) {
		// Error  - backend address pool wasn't added to the app gateway
		resp.Diagnostics.AddError(
			"Unable to create Backend Address pool ######## API response = "+fmt.Sprint(code)+"\n"+ress_error, //+args+ress_gw+"\n"
			"Backend Address pool Name doesn't exist in the response app gateway",
		)
		return
	}
	// log the added backend address pool
	i := getBackendAddressPoolElementKey(gw_response, backend_json.Name)
	tflog.Trace(ctx, "created BackendAddressPool", "BackendAddressPool ID", gw_response.Properties.BackendAddressPools[i].ID)

	// Map response body to resource schema attribute
	backend_state := Backend_address_pool{
		Name:         types.String{Value: gw_response.Properties.BackendAddressPools[i].Name},
		Id:           types.String{Value: gw_response.Properties.BackendAddressPools[i].ID},
		Fqdns:        []types.String{},
		Ip_addresses: []types.String{},
	}
	backend_state.Fqdns = make([]types.String, 1)
	backend_state.Ip_addresses = make([]types.String, 1)
	backend_state.Fqdns[0] = types.String{Value: gw_response.Properties.BackendAddressPools[i].Properties.BackendAddresses[0].Fqdn}
	backend_state.Ip_addresses[0] = types.String{Value: gw_response.Properties.BackendAddressPools[i].Properties.BackendAddresses[1].IPAddress}

	// Generate resource state struct
	var result = WebappBinding{
		Name:                 plan.Name,
		Agw_name:             types.String{Value: gw_response.Name},
		Agw_rg:               plan.Agw_rg,
		Backend_address_pool: backend_state,
	}
	//store to the created objecy to the terraform state
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

	// Get gw from API and then update what is in state from what the API returns
	webappBindingName := state.Name.Value

	//Get the agw
	resourceGroupName := state.Agw_rg.Value
	applicationGatewayName := state.Agw_name.Value
	gw := getGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, r.p.token.Access_token)
	//test if the backend address pool doen't exist in the gateway, then it is an error
	if !checkBackendAddressPoolElement(gw, state.Backend_address_pool.Name.Value) {
		// Error  - the non existance of backend_plan address pool name must stop execution
		resp.Diagnostics.AddError(
			"Unable to read Backend Address pool",
			"Backend Address pool Name doesn't exist in the app gateway. ###Certainly, it was removed manually###",
		)
		return
	}
	// Get the current backend address pool from the API
	backend_json := gw.Properties.BackendAddressPools[getBackendAddressPoolElementKey(gw, state.Backend_address_pool.Name.Value)]

	// Map response body to resource schema attribute
	backend_state := Backend_address_pool{
		Name:         types.String{Value: backend_json.Name},
		Id:           types.String{Value: backend_json.ID},
		Fqdns:        []types.String{},
		Ip_addresses: []types.String{},
	}
	backend_state.Fqdns = make([]types.String, 1)
	backend_state.Ip_addresses = make([]types.String, 1)
	backend_state.Fqdns[0] = types.String{Value: backend_json.Properties.BackendAddresses[0].Fqdn}
	backend_state.Ip_addresses[0] = types.String{Value: backend_json.Properties.BackendAddresses[1].IPAddress}

	// Generate resource state struct
	var result = WebappBinding{
		Name:                 types.String{Value: webappBindingName},
		Agw_name:             state.Agw_name,
		Agw_rg:               state.Agw_rg,
		Backend_address_pool: backend_state,
	}

	state = result
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

// Update resource
func (r resourceWebappBinding) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {

	// Get plan values
	var plan WebappBinding
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state WebappBinding
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get the agw in order to update it with new values from plan
	resourceGroupName := plan.Agw_rg.Value
	applicationGatewayName := plan.Agw_name.Value
	gw := getGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, r.p.token.Access_token)

	//Verify if the agw already contains the wanted element
	var backend_plan Backend_address_pool
	backend_plan = plan.Backend_address_pool
	//resp.Diagnostics.AddWarning("################ Backend Address Pool Name: ", backend_plan.Name.Value)
	if !checkBackendAddressPoolElement(gw, backend_plan.Name.Value) {
		// Error  - existing backend_plan address pool name must stop execution
		resp.Diagnostics.AddError(
			"Unable to update the Backend Address pool",
			"Backend Address pool Name dosen't exist in the app gateway",
		)
		return
	}

	//create and map the new backend_json object from the backend_plan
	backend_json := azureagw.BackendAddressPool{
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

	//remove the old backend from the gateway
	removeBackendAddressPoolElement(&gw, backend_json.Name)
	//add the new one
	gw.Properties.BackendAddressPools = append(gw.Properties.BackendAddressPools, backend_json)
	//and update the gateway
	gw_response, responseData, code := updateGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, gw, r.p.token.Access_token)

	//if there is an error, responseData contains the error message in jason, else, gw_response is a correct gw Object
	rs := string(responseData)
	ress_error, err := PrettyString(rs)
	if err != nil {
		log.Fatal(err)
	}

	//verify if the backend address pool is added to the gateway
	if !checkBackendAddressPoolElement(gw_response, backend_json.Name) {
		// Error  - backend address pool wasn't added to the app gateway
		resp.Diagnostics.AddError(
			"Unable to create Backend Address pool ######## API response code="+fmt.Sprint(code)+"\n"+ress_error, //+args+ress_gw+"\n"
			"Backend Address pool Name doesn't exist in the response app gateway",
		)
		return
	}

	// log the added backend address pool
	i := getBackendAddressPoolElementKey(gw_response, backend_json.Name)
	tflog.Trace(ctx, "Updated BackendAddressPool", "BackendAddressPool ID", gw_response.Properties.BackendAddressPools[i].ID)

	// Map response body to resource schema attribute
	backend_state := Backend_address_pool{
		Name:         types.String{Value: gw_response.Properties.BackendAddressPools[i].Name},
		Id:           types.String{Value: gw_response.Properties.BackendAddressPools[i].ID},
		Fqdns:        []types.String{},
		Ip_addresses: []types.String{},
	}
	backend_state.Fqdns = make([]types.String, 1)
	backend_state.Ip_addresses = make([]types.String, 1)
	backend_state.Fqdns[0] = types.String{Value: gw_response.Properties.BackendAddressPools[i].Properties.BackendAddresses[0].Fqdn}
	backend_state.Ip_addresses[0] = types.String{Value: gw_response.Properties.BackendAddressPools[i].Properties.BackendAddresses[1].IPAddress}

	// Generate resource state struct
	var result = WebappBinding{
		Name:                 state.Name,
		Agw_name:             types.String{Value: gw_response.Name},
		Agw_rg:               state.Agw_rg,
		Backend_address_pool: backend_state,
	}
	//store to the created objecy to the terraform state
	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete resource
func (r resourceWebappBinding) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	// Get current state
	var state WebappBinding
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Get backend address pool name from state
	backend_name := state.Backend_address_pool.Name.Value
	resp.Diagnostics.AddWarning("################ Delete Backend Address Pool Name: ", backend_name)

	//Get the agw
	resourceGroupName := state.Agw_rg.Value
	applicationGatewayName := state.Agw_name.Value
	gw := getGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, r.p.token.Access_token)
	//test if the backend address pool doen't exist in the gateway, then it is an error
	if !checkBackendAddressPoolElement(gw, backend_name) {
		// Error  - the non existance of backend_plan address pool name must stop execution
		resp.Diagnostics.AddError(
			"Unable to delete Backend Address pool",
			"Backend Address pool Name doesn't exist in the app gateway. ###Certainly, it was removed manually###",
		)
		return
	}

	//remove the backend from the gw
	removeBackendAddressPoolElement(&gw, backend_name)

	//and update the gateway
	gw_response, responseData, code := updateGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, gw, r.p.token.Access_token)

	//if there is an error, responseData contains the error message in jason, else, gw_response is a correct gw Object
	rs := string(responseData)
	ress_error, err := PrettyString(rs)
	if err != nil {
		log.Fatal(err)
	}
	resp.Diagnostics.AddWarning("----------------- API code: "+fmt.Sprint(code)+"\n", ress_error)

	//verify if the backend address pool is added to the gateway
	if code!= 200 {//checkBackendAddressPoolElement(gw_response, backend_name) {
		// Error  - backend address pool wasn't added to the app gateway
		resp.Diagnostics.AddError(
			"Unable to delete Backend Address pool ######## API response code="+fmt.Sprint(code)+"\n"+ress_error, //+args+ress_gw+"\n"
			"Backend Address pool Name still exist in the response app gateway",
		)
		return
	}

	// Remove resource from state
	resp.State.RemoveResource(ctx)
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
	var agw azureagw.ApplicationGateway
	err = json.Unmarshal(responseData, &agw)

	if err != nil {
		fmt.Println(err)
	}
	return agw
}
func updateGW(subscriptionId string, resourceGroupName string, applicationGatewayName string, gw azureagw.ApplicationGateway, token string) (azureagw.ApplicationGateway, []byte, int) {
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
	code := resp.StatusCode
	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var agw azureagw.ApplicationGateway
	err = json.Unmarshal(responseData, &agw)

	if err != nil {
		fmt.Println(err)
	}
	return agw, responseData, code
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
