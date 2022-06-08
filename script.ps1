param(
    [Parameter(Mandatory = $true)]
    [string]$Backendpool = "default-citeo-plus-be-pool"
)

$AppGatewayName= "default-app-gateway-mahmoud"
$gw = Get-AzApplicationGateway -Name ${AppGatewayName} -ResourceGroupName "shared-app-gateway"
Get-AzApplicationGatewayBackendAddressPool -ApplicationGateway ${gw} -Name ${Backendpool} -ErrorAction "SilentlyContinue"


function Make-BackendPool($app) {
    $name=$app.webapp+"-be-pool"+$app.uniq
    $bepool=Get-AzApplicationGatewayBackendAddressPool -ApplicationGateway ${Gw} -Name "$name" -ErrorAction "SilentlyContinue"
    if ( -not $bepool) {
        Write-Host "new backendpool ${name}"
        $existing=Add-AzApplicationGatewayBackendAddressPool -ApplicationGateway $gw -Name "$Name"
    } else {
        Write-Host " got backendpool"$name
    }
    return $name 
}