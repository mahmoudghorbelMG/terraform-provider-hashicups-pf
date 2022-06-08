param(
    [Parameter(Mandatory = $true)]
    [string]$Backendpool = "default-citeo-plus-be-pool"
)

$AppGatewayName= "default-app-gateway-mahmoud"
$gw = Get-AzApplicationGateway -Name ${AppGatewayName} -ResourceGroupName "shared-app-gateway"
Get-AzApplicationGatewayBackendAddressPool -ApplicationGateway ${gw} -Name ${Backendpool} -ErrorAction "SilentlyContinue"
