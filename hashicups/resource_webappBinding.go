package hashicups

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
						Optional: true,
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

	//Get the agw (app gateway) from Azure with its Rest API
	resourceGroupName := plan.Agw_rg.Value
	applicationGatewayName := plan.Agw_name.Value
	gw := getGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, r.p.token.Access_token)

	//Check if the agw already contains an element that has the same name
	checkElementName(gw,plan,resp)
	
	//create and map the new Backend pool element (backend_json) object from the plan (backend_plan)
	createBackendAddressPool(&gw,plan.Backend_address_pool)
		
	gw_response, responseData, code := updateGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, gw, r.p.token.Access_token)
	//if there is an error, responseData contains the error message in jason, else, gw_response is a correct gw Object
	rs := string(responseData)
	ress_error, err := PrettyString(rs)
	if err != nil {
		log.Fatal(err)
	}
	
	//verify if the backend address pool is added to the gateway, otherwise exit error
	checkCreatedBackendAddressPool(gw_response,plan.Backend_address_pool,resp,code,ress_error)
	
	//generate BackendState
	backend_state:= generateBackendAddressPoolState(gw_response,plan.Backend_address_pool)
	
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

	//backend_state2 :=generateBackendAddressPoolState(gw, state.Backend_address_pool)
	// Get the current backend address pool from the API
	backend_json := gw.Properties.BackendAddressPools[getBackendAddressPoolElementKey(gw, state.Backend_address_pool.Name.Value)]

	// Map response body to resource schema attribute
	backend_state := Backend_address_pool{
		Name:         types.String{Value: backend_json.Name},
		Id:           types.String{Value: backend_json.ID},
		Fqdns:        []types.String{},
		Ip_addresses: []types.String{},
	}

	length_BackendAddresses := len(backend_json.Properties.BackendAddresses)
	fmt.Println("tttttttttttttttttt  length_BackendAddresses = ",length_BackendAddresses)
	length_Fqdns :=0	
	for i := 0; i < length_BackendAddresses; i++ {
		if (backend_json.Properties.BackendAddresses[i].Fqdn != "")&&(&backend_json.Properties.BackendAddresses[i].Fqdn != nil) {			
			length_Fqdns++ 
		}else{
			fmt.Println("tttttttttttttttttt   backend_json.Properties.BackendAddresses[i].Fqdn = ''  ou nil:")
		}
	}
	length_Ip := length_BackendAddresses - length_Fqdns

	if length_Fqdns != 0 {
		backend_state.Fqdns = make([]types.String, length_Fqdns)
	}else{
		backend_state.Fqdns = nil
	}
	
	if length_Ip != 0 {
		backend_state.Ip_addresses = make([]types.String, length_Ip)
	}else{
		backend_state.Ip_addresses = nil
	}
	

	for j := 0; j < length_Fqdns; j++ {
        backend_state.Fqdns[j]= types.String{Value: backend_json.Properties.BackendAddresses[j].Fqdn}
    }
	for j := 0; j < length_Ip; j++ {
        backend_state.Ip_addresses[j] = types.String{Value: backend_json.Properties.BackendAddresses[j+length_Fqdns].IPAddress}
    }
/*
	backend_state.Fqdns = make([]types.String, 1)
	backend_state.Ip_addresses = make([]types.String, 1)
	backend_state.Fqdns[0] = types.String{Value: backend_json.Properties.BackendAddresses[0].Fqdn}
	backend_state.Ip_addresses[0] = types.String{Value: backend_json.Properties.BackendAddresses[1].IPAddress}*/

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
				ID string "json:\"id,omitempty\""
			} "json:\"requestRoutingRules,omitempty\""
		}{},
		Type: "Microsoft.Network/applicationGateways/backendAddressPools",
	}

	length := len(backend_plan.Fqdns)+len(backend_plan.Ip_addresses)
	if length != 0 {
		backend_json.Properties.BackendAddresses = make([]struct {
			Fqdn      string "json:\"fqdn,omitempty\""
			IPAddress string "json:\"ipAddress,omitempty\""
		}, length)
	}else{
		backend_json.Properties.BackendAddresses = nil
	}
	

	for i := 0; i < len(backend_plan.Fqdns); i++ {
        backend_json.Properties.BackendAddresses[i].Fqdn = backend_plan.Fqdns[i].Value
    }
	for i := 0; i < len(backend_plan.Ip_addresses); i++ {
        backend_json.Properties.BackendAddresses[i+len(backend_plan.Fqdns)].IPAddress = backend_plan.Ip_addresses[i].Value
    }
/*
	backend_json.Properties.BackendAddresses = make([]struct {
		Fqdn      string "json:\"fqdn,omitempty\""
		IPAddress string "json:\"ipAddress,omitempty\""
	}, 2)
	backend_json.Properties.BackendAddresses[0].Fqdn = backend_plan.Fqdns[0].Value
	backend_json.Properties.BackendAddresses[1].IPAddress = backend_plan.Ip_addresses[0].Value*/

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
	//verify if the new backend address pool is added to the gateway
	if !checkBackendAddressPoolElement(gw_response, backend_json.Name) {
		// Error  - backend address pool wasn't added to the app gateway
		resp.Diagnostics.AddError(
			"Unable to create Backend Address pool ######## API response code="+fmt.Sprint(code)+"\n"+ress_error, //+args+ress_gw+"\n"
			"Backend Address pool Name doesn't exist in the response app gateway",
		)
		return
	}

	// log the added backend address pool
	index := getBackendAddressPoolElementKey(gw_response, backend_json.Name)
	backend_json2 := gw_response.Properties.BackendAddressPools[index]
	tflog.Trace(ctx, "Updated BackendAddressPool", "BackendAddressPool ID", backend_json2.ID)

	// Map response body to resource schema attribute
	backend_state := Backend_address_pool{
		Name:         types.String{Value: backend_json2.Name},
		Id:           types.String{Value: backend_json2.ID},
		Fqdns:        []types.String{},
		Ip_addresses: []types.String{},
	}

	length_Backends := len(backend_json2.Properties.BackendAddresses)
	length_Fqdns :=0	
	for i := 0; i < length_Backends; i++ {
		if !(backend_json2.Properties.BackendAddresses[i].Fqdn == "") {
			length_Fqdns++ 
		}
	}
	length_Ip := length_Backends - length_Fqdns

	if length_Fqdns != 0 {
		backend_state.Fqdns = make([]types.String, length_Fqdns)
	}else{
		backend_state.Fqdns = nil
	}

	if length_Ip != 0 {
		backend_state.Ip_addresses = make([]types.String, length_Ip)
	}else{
		backend_state.Ip_addresses = nil
	}
	
	for j := 0; j < length_Fqdns; j++ {
        backend_state.Fqdns[j]= types.String{Value: backend_json2.Properties.BackendAddresses[j].Fqdn}
    }
	for j := 0; j < length_Ip; j++ {
        backend_state.Ip_addresses[j] = types.String{Value: backend_json2.Properties.BackendAddresses[j+length_Fqdns].IPAddress}
    }
	
	
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
	_, responseData, code := updateGW(r.p.AZURE_SUBSCRIPTION_ID, resourceGroupName, applicationGatewayName, gw, r.p.token.Access_token)
	//if there is an error, responseData contains the error message in jason, else, gw_response is a correct gw Object
	rs := string(responseData)
	ress_error, err := PrettyString(rs)
	if err != nil {
		log.Fatal(err)
	}
	resp.Diagnostics.AddWarning("----------------- API code: "+fmt.Sprint(code)+"\n", "ress_error")
	//exit :=checkBackendAddressPoolElement(gw_response, backend_name)
	//verify if the backend address pool is added to the gateway
	if code != 200 { //checkBackendAddressPoolElement(gw_response, backend_name) {
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
func checkElementName(gw azureagw.ApplicationGateway, plan WebappBinding,resp *tfsdk.CreateResourceResponse){
	//This function allows to check if an element name in the required new configuration (plan WebappBinding) already exist in the gw.
	//if so, the provider has to stop executing and issue an exit error

	//Create new var for all configurations
	backend_plan := plan.Backend_address_pool	

	if checkBackendAddressPoolElement(gw, backend_plan.Name.Value) {
		// Error  - existing backend_plan address pool name must stop execution
		resp.Diagnostics.AddError(
			"Unable to create Backend Address pool",
			"Backend Address pool Name already exists in the app gateway",
		)
		return
	}
}

//Backend pool operations
func createBackendAddressPool(gw *azureagw.ApplicationGateway, backend_plan Backend_address_pool) {
	backend_json := azureagw.BackendAddressPool{
		Name: backend_plan.Name.Value,
		Properties: struct {
			ProvisioningState string "json:\"provisioningState,omitempty\""
			BackendAddresses  []struct {
				Fqdn      string "json:\"fqdn,omitempty\""
				IPAddress string "json:\"ipAddress,omitempty\""
			} "json:\"backendAddresses\""
			RequestRoutingRules []struct {
				ID string "json:\"id,omitempty\""
			} "json:\"requestRoutingRules,omitempty\""
		}{},
		Type: "Microsoft.Network/applicationGateways/backendAddressPools",
	}
	length := len(backend_plan.Fqdns)+len(backend_plan.Ip_addresses)
	
	//If there is no fqdn nor IPaddress for the backend pool, initialize the BackendAddresses to nil to avoid a terraform provider error when making the state
	if length == 0 {
		backend_json.Properties.BackendAddresses = nil
	}else{
		backend_json.Properties.BackendAddresses = make([]struct {
			Fqdn      string "json:\"fqdn,omitempty\""
			IPAddress string "json:\"ipAddress,omitempty\""
		}, length)	
	}	
	for i := 0; i < len(backend_plan.Fqdns); i++ {
        backend_json.Properties.BackendAddresses[i].Fqdn = backend_plan.Fqdns[i].Value
    }
	for i := 0; i < len(backend_plan.Ip_addresses); i++ {
        backend_json.Properties.BackendAddresses[i+len(backend_plan.Fqdns)].IPAddress = backend_plan.Ip_addresses[i].Value
    }	
	// add the backend to the agw and update the agw
	gw.Properties.BackendAddressPools = append(gw.Properties.BackendAddressPools, backend_json)
}
func checkCreatedBackendAddressPool(gw_response azureagw.ApplicationGateway, backend_plan Backend_address_pool,resp *tfsdk.CreateResourceResponse, code int, ress_error string) {
	if !checkBackendAddressPoolElement(gw_response, backend_plan.Name.Value) {
		// Error  - backend address pool wasn't added to the app gateway
		resp.Diagnostics.AddError(
			"Unable to create Backend Address pool ######## API response = "+fmt.Sprint(code)+"\n"+ress_error, //+args+ress_gw+"\n"
			"Backend Address pool Name doesn't exist in the response of the app gateway",
		)
		return
	}
}
func generateBackendAddressPoolState(gw_response azureagw.ApplicationGateway, backend_plan Backend_address_pool) Backend_address_pool {
	index := getBackendAddressPoolElementKey(gw_response, backend_plan.Name.Value)
	backend_json:=gw_response.Properties.BackendAddressPools[index]
	// log the added backend address pool
	//tflog.Trace(ctx, "created BackendAddressPool", "BackendAddressPool ID", backend_json.ID)

	// Map response body to resource schema attribute
	backend_state := Backend_address_pool{
		Name:         types.String{Value: backend_json.Name},
		Id:           types.String{Value: backend_json.ID},
		Fqdns:        []types.String{},
		Ip_addresses: []types.String{},
	}
	fmt.Println("------------------ The number len(backend_plan.Fqdns) is:", len(backend_plan.Fqdns))
	
	if len(backend_plan.Fqdns) != 0 {
		backend_state.Fqdns = make([]types.String, len(backend_plan.Fqdns))
	}else{
		backend_state.Fqdns = nil
	}
	fmt.Println("------------------ The number len(backend_plan.Ip_addresses) is:", len(backend_plan.Ip_addresses))
	
	if len(backend_plan.Ip_addresses) != 0 {
		backend_state.Ip_addresses = make([]types.String, len(backend_plan.Ip_addresses))
	}else{
		backend_state.Ip_addresses = nil
	}

	for j := 0; j < len(backend_plan.Fqdns); j++ {
        backend_state.Fqdns[j]= types.String{Value: backend_json.Properties.BackendAddresses[j].Fqdn}
    }
	for j := 0; j < len(backend_plan.Ip_addresses); j++ {
		backend_state.Ip_addresses[j] = types.String{Value: backend_json.Properties.BackendAddresses[j+len(backend_plan.Fqdns)].IPAddress}
    }

	return backend_state
}
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

//some debugging tools
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
func PrettyStringFromByte(str []byte) (string, error) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, str, "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}
func PrettyString(str string) (string, error) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}
func printToFile(str string, fileName string){
	file, err := os.Create(fileName)
    if err != nil {
        log.Fatal(err)
    }
    mw := io.MultiWriter(os.Stdout, file)
    fmt.Fprintln(mw, str)
}