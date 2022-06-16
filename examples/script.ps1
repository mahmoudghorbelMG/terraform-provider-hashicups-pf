param(
    [Parameter(Mandatory = $true)]
    [string]$Backendpool = "default-citeo-plus-be-pool"
)
Select-AzSubscription  "Citeo Dev/Test"
$AppGatewayName= "default-app-gateway-mahmoud"
$gw = Get-AzApplicationGateway -Name ${AppGatewayName} -ResourceGroupName "shared-app-gateway"
Get-AzApplicationGatewayBackendAddressPool -ApplicationGateway ${gw} -Name ${Backendpool} -ErrorAction "SilentlyContinue"


# initialisation commands -SubscriptionName
#pwsh -Command Connect-AzAccount -UseDeviceAuthentication
#pwsh -Command Set-AzContext b3ae2f08-8ccb-4640-949e-b4c0d2acfde6  # subscription Citeo Dev/Test


#pwsh -Command $gw=Get-AzApplicationGateway -Name "default-app-gateway-mahmoud" -ResourceGroupName "shared-app-gateway" -Debug


