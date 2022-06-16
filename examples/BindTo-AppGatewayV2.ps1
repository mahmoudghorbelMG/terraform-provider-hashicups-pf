param(
    [Parameter(Mandatory = $true)]
    $WebAppDatas,                           # data, format json
    [string]$LEEnv="LE_PROD",               # Environnement let's encrypt � utiliser (LE_STAGE ou LE_PROD)
    [string]$AppGatewayName="app-gateway"   # App Gateway � utiliser
)

$leaseId=""
#Import-Module Posh-ACME
$accountMail="devops@ecoemballages.onmicrosoft.com"
$ec=0
$pip= Get-AzPublicIpAddress -name "${AppGatewayName}-public-ip"
$subPrefix=(get-azsubscription).id.split('-')[0]

# Les infos pour r�cup�rer le zip contenant les certifs
$PAResourceGroupName="shared-storage"
$PAStorageAccountName="citeoartifact${subPrefix}"
$PAContainerName="certificates"
$PABlobName="certificates.zip"
$PAAccountId=102731797

$scriptHome=Get-Location
if ($Env:LOCALAPPDATA) {
  # Windows
  $PAHome="${Env:LOCALAPPDATA}/Posh-ACME"
} else {
  # Unix
  $PAHome="${Env:HOME}/.config/Posh-ACME"
}


# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
function lockMe() {
  $SasToken=@{}
  $SasToken.add("7cc5611c","?sv=2019-12-12&ss=bfqt&srt=sco&sp=rwdlacupx&se=2023-02-05T22:10:34Z&st=2021-02-05T14:10:34Z&spr=https&sig=7WCtRpLmVdStYTtyQkqcMuOfhAucsJhjCVlPnfzLZR0%3D")
  $SasToken.add("b3ae2f08","?sv=2020-02-10&ss=bfqt&srt=sco&sp=rwdlacupx&se=2025-03-07T17:14:16Z&st=2021-03-07T09:14:16Z&spr=https,http&sig=7tCiILHxoAo7andsGBE4L2gqkwvk4l%2BVo1mDrcmArPU%3D")

  $storageCtx = New-AzStorageContext -SasToken $SasToken[$subPrefix] -StorageAccountName "citeoiacstate${subPrefix}"
  $blob = Get-AzStorageBlob -Container "terraform-state" -Blob "bind-appgw-lock" -Context $storageCtx
  if (-Not $blob) {
      throw "Pas pu attraper le blob. Mauvais SAS Token ?"
  }
  do {
      Write-Host "* Acquiring lease"
      try {
          $leaseId = $blob.ICloudBlob.AcquireLease($null,$null)
      } catch {
          Write-Host " Already leased, sleep for ~25s"
      } finally {
          sleep (Get-Random -Minimum 20 -Maximum 30)
      }
  } while ($leaseId -eq "")
  Write-Host "Lease acquired"
  return $blob,$leaseId
}

function unlockMe($blob, $leaseId) {
  Write-Host "* Releasing lease"
  $accessCondition = New-Object Microsoft.Azure.Storage.AccessCondition
  $accessCondition.leaseId=$LeaseId
  $blob.ICloudBlob.ReleaseLease($accessCondition)
}

function Get-PfxPass() {
    Write-Host "getting pfx pass"
    $subPrefix=(get-azsubscription).id.split('-')[0]
    try {
        $pfxPass=Get-AzKeyVaultSecret -VaultName "citeo-shared-${subPrefix}" -Name "letsencrypt-pfxpass" -AsPlainText -ErrorAction "Stop"
    } catch {
        throw "Pas pu r�cup�rer le pfx pass"
    }
    return $pfxPass
}

function Make-HttpSettings($app) {
    $name       = $app.webapp+"-settings-"+$app.port+$app.uniq
    $existing   = Get-AzApplicationGatewayBackendHttpSetting -ApplicationGateway $gw -Name "$name" -ErrorAction "SilentlyContinue"

    if (-not $existing){
        Write-Host  "new httpSettings $name"
        $existing =Add-AzApplicationGatewayBackendHttpSetting -ApplicationGateway $gw -Name "$name" -Port $app.port -Protocol $app.protocol -CookieBasedAffinity Disabled
    }else {
        Write-Host " got httpSettings"$name
    }
    return $name
}

function Make-Probes($app) {
    $name       = $app.webapp+"-probe-"+$app.port+$app.uniq
    $existing   = Get-AzApplicationGatewayProbeConfig -Name "$name" -ApplicationGateway $gw -ErrorAction "SilentlyContinue"
    if (-not $existing){
        Write-Host "new probe $name"
        $existing=Add-AzApplicationGatewayProbeConfig -Name "${name}" -ApplicationGateway $gw -Protocol $app.protocol -Path $app.probePath -Interval 30 -Timeout 30 -UnhealthyThreshold 3 -PickHostNameFromBackendHttpSettings -Match $probeMatch
    } else {
        Write-Host " got probe"$name
    }
    return $name
}

function Link-Probe-HttpSettings($app) {
    $Probe = Get-AzApplicationGatewayProbeConfig -Name $app.probeName -ApplicationGateway $gw -ErrorAction "SilentlyContinue"
    Set-AzApplicationGatewayBackendHttpSettings -Name $app.httpSettingsName -ApplicationGateway $gw -PickHostNameFromBackendAddress -Port $app.port -Protocol $app.protocol -CookieBasedAffinity Disabled -RequestTimeout 666 -Probe $Probe
}

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

function Make-FEPort($app) {
    $feports=Get-AzApplicationGatewayFrontendPort -ApplicationGateway $gw
    ForEach ($port in $feports) {
        if($port.port -eq $app.port) {
            Write-Host " got frontend port"$port.name
            $name=$port.name
        }
    }
    if (-not $name) {
        $name="${AppGatewayName}-fe-port-"+$app.port+$app.uniq
        Write-Host "new frontend port"$name
        $a=Add-AzApplicationGatewayFrontendPort -ApplicationGateway ${Gw} -Name "$name" -Port $app.port
    }
    return $name
}

function Make-FEIpConfig ($app) {
    $name=$app.webapp+"-fe-ip-config"+$app.uniq
    $existing=Get-AzApplicationGatewayFrontendIPConfig -ApplicationGateway $gw
    if ($existing.count -eq 0) {
        Write-Host "new frontend ip config $name"
        $void=Add-AzApplicationGatewayFrontendIPConfig -ApplicationGateway $gw -Name $name -PublicIPAddress $pip -ErrorAction "SilentlyContinue"
    } else {
        Write-Host " got frontend ip config"$existing.name
        return $existing.name
    }
    return $name
}

function Make-Listener($app) {
    $feport=Get-AzApplicationGatewayFrontendPort -ApplicationGateway $gw -Name $app.feportName
    $fip=Get-AzApplicationGatewayFrontendIPConfig -ApplicationGateway $gw -Name $app.fipName
    $name=$app.webapp+"-listener-"+$app.port+$app.uniq
    $existing=Get-AzApplicationGatewayHttpListener -ApplicationGateway $gw -Name $name -ErrorAction "SilentlyContinue"
    if (-not $existing) {
        Write-Host "new listener $name"
        if ($app.port -eq 443) {
            if (-Not $app.certname) {
                $certname=$app.webapp+$app.uniq+"-cert"
            } else {
                $certname=$app.certname
            }
            $cert=Get-AzApplicationGatewaySslCertificate -ApplicationGateway $gw -Name "$certname" -ErrorAction "SilentlyContinue"
            if (-not $cert) {
                Write-Host "new cert"
                $cert=Set-Cert($_)
            }
            $existing=Add-AzApplicationGatewayHttpListener -ApplicationGateway $gw -Name $name -FrontendIPConfiguration $fip -FrontendPort $feport -HostName $app.fqdn -Protocol $app.protocol -SslCertificate $cert
        } else {
            $existing=Add-AzApplicationGatewayHttpListener -ApplicationGateway $gw -Name $name -FrontendIPConfiguration $fip -FrontendPort $feport -HostName $app.fqdn -Protocol $app.protocol
        }
    } else {
        Write-Host " got listener"$name
    }
    return $name
}

function Make-Rules($app) {
    $name=$app.webapp+"-rule-"+$app.port+$app.uniq
    $existing=Get-AzApplicationGatewayRequestRoutingRule -ApplicationGateway $gw -Name $name -ErrorAction "SilentlyContinue"
    if ( -not $existing ) {
        Write-Host "new rule $name"
        $httpSettings=Get-AzApplicationGatewayBackendHttpSetting -ApplicationGateway $gw -Name $app.httpSettingsName
        $listener=Get-AzApplicationGatewayHttpListener -ApplicationGateway $gw -Name $app.listenerName
        $beapool=Get-AzApplicationGatewayBackendAddressPool -ApplicationGateway $gw -Name $app.BEAPoolName

        $existing=Add-AzApplicationGatewayRequestRoutingRule -ApplicationGateway $gw -Name $name -RuleType "Basic" -BackendHttpSettings $httpSettings -HttpListener $listener -BackendAddressPool $beapool
    } else {
        Write-Host " got rule"$name
    }
}

function Get-Gandi-API-Key() {
    $kv="citeo-shared-${subPrefix}"
    $k=(Get-AzKeyVaultSecret -vaultName "${kv}" -name "gandi-api-key").secretValue
    if ($k -eq $null) {
        write-error "Gandi Key not found. KV: $kv"
        return $false
    } else {
        return $k
    }
}

function Set-Cert($app) {
    $certname=$app.webapp+$app.uniq+"-cert"
    $pfxPass=Get-PfxPass
    $Password=ConvertTo-SecureString "$pfxPass" -AsPlainText -Force
    $cert=Get-AzApplicationGatewaySslCertificate -ApplicationGateway $gw -Name $certname -ErrorAction "SilentlyContinue"
    if (-not $cert) {
        Write-Host "new cert"$certname
        $builtCert=Make-Cert -fqdn $app.fqdn
        $pfx=$builtCert.PfxFullChain
        $void=Add-AzApplicationGatewaySslCertificate -ApplicationGateway $gw -Name $certname -CertificateFile $pfx -Password $Password
        $cert=Get-AzApplicationGatewaySslCertificate -ApplicationGateway $gw -Name $certname -ErrorAction "SilentlyContinue"
    } else {
        Write-Host " got cert"$certname
    }
    return $cert
}

function Make-Cert($fqdn) {
    $GandiApiKey=Get-Gandi-API-Key
    if (-not (Get-Module -Name Posh-ACME)) {
        Import-Module -force Posh-ACME
    }
    Set-PAServer $LEEnv

    $gParams = @{GandiToken=$GandiApiKey}
    $cert=Get-PACertificate -MainDomain "$fqdn"
    if (-not $cert) {
        Write-Host "new cert for"${fqdn}
        $pfxPass=Get-PfxPass
        $void=New-PACertificate $fqdn -AcceptTOS -Contact $accountMail -DnsPlugin Gandi -PluginArgs $gParams -Force -PfxPass $pfxPass
        $cert=Get-PACertificate -MainDomain "$fqdn"
    }
    return $cert
}

function Make-Redirection($app) {
    $a=$app
    $a.port="80"
    $a.protocol="http"
    $FEPortResults = Make-FEPort($a)
    $a.feportName=$FEPortResults
    $FEIpResult = Make-FEIpConfig($a)
    $a.fipName=$FEIpResult
    $httpListenerName=Make-Listener($a)
    $httpListener=Get-AzApplicationGatewayHttpListener -ApplicationGateway $gw -Name $httpListenerName
    $name=$app.webapp+"-redirect-"+$a.port+$app.uniq
    $httpsListener=Get-AzApplicationGatewayHttpListener -ApplicationGateway $gw -Name $app.listenerName
    $redirect=Get-AzApplicationGatewayRedirectConfiguration -ApplicationGateway $gw -Name $name -ErrorAction "SilentlyContinue"
    if ( -not $redirect ) {
        Write-Host "new redirect $name"
        Add-AzApplicationGatewayRedirectConfiguration -ApplicationGateway $gw -Name $name -RedirectType "Permanent" -TargetListener $httpsListener
        $redirect=Get-AzApplicationGatewayRedirectConfiguration -ApplicationGateway $gw -Name $name
    } else {
        Write-Host " got redirect"$name
    }
    $name=$app.webapp+"-rule-"+$a.port+$app.uniq
    $rule=Get-AzApplicationGatewayRequestRoutingRule -ApplicationGateway $gw -Name $name -ErrorAction "SilentlyContinue"
    if ( -not $rule ) {
        Write-Host "new rule $name"
        $rule=Add-AzApplicationGatewayRequestRoutingRule -ApplicationGateway $gw -Name $name -RuleType "Basic" -RedirectConfiguration $redirect -HttpListener $httpListener
    } else {
        Write-Host " got rule"$name
    }
}

function GetLEData() {
  Write-Host -NoNewline "* Fetching PA archive: "
  $storageAcc=Get-AzStorageAccount -ResourceGroupName "$PAResourceGroupName" -Name "$PAStorageAccountName"
  $ctx=$storageAcc.Context
  Write-Host -NoNewline "download "
  $void=Get-AzStorageBlobContent -Context $ctx -Container "$PAContainerName" -Blob "$PABlobName" -Destination "$scriptHome/$PABlobName"
  Write-Host -NoNewline "unzip "
  $void=Expand-Archive -Path "$scriptHome/$PABlobName" -DestinationPath "${PAHome}" -Force
  Write-Host
  $void=Remove-Item -LiteralPath "$scriptHome/$PABlobName"
}

function PushLEData() {
  Write-Host -NoNewline "* Pushing PA archive to storage: "
  $storageAcc=Get-AzStorageAccount -ResourceGroupName "$PAResourceGroupName" -Name "$PAStorageAccountName"
  $ctx=$storageAcc.Context
  Write-Host -NoNewline "zip "
  $here=Get-Location
  $void=Set-Location "${PAHome}"
  $void=Compress-Archive -Path "${LEEnv}" -DestinationPath "$scriptHome/$PABlobName"
  Set-Location "$here"
  Write-Host -NoNewline "upload "
  $void=Set-AzStorageBlobContent -Context $ctx -Container "$PAContainerName" -File "$scriptHome/$PABlobName" -Force
  Write-Host
  $void=Remove-Item -LiteralPath "${PAHome}/${LEEnv}" -Recurse
  $void=Remove-Item -LiteralPath "$scriptHome/$PABlobName"
  # Recr�er le r�pertoire, sinon un prochain appel au script plante avec un WARNING: Unable to find cached PAServer info for https://acme-v02.api.letsencrypt.org/directory. Try using Set-PAServer first.
  $void=Set-PAServer $LEEnv |  Out-Null
}
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
$blob,$leaseId=lockMe

$gw = Get-AzApplicationGateway -Name ${AppGatewayName} -ResourceGroupName "shared-app-gateway"
$WebApps = $WebAppDatas | ConvertFrom-Json
$probeMatch      = New-AzApplicationGatewayProbeHealthResponseMatch -StatusCode 200-399

$WarningPreference = "SilentlyContinue"
# WebAppDatas is a json like :
# [ {
#   "app":"bo of guidedutri",             # chaine qui identifie cette conf
#   "fqdn":"bo.default.guidedutri.fr",    # fqdn sur lequel doit répondre l'appgw pour cette conf
#   "port":443,                           # port sur lequel écouter
#   "probePath":"/probe",                 # url à laquelle l'appgw doit se connecter pour déterminer si la webapp est vivante ou pas. DOIT répondre 2xx
#   "webapp":"default-citeo-gdtr",        # Nom Azure de la webapp
#   "uniq":"-bo",                         # optionel: si vous voulez que plusieurs fqdn tapent sur la meme web app, il faut mettre cette valeur à quelque chose de différent pour chaque boc de conf ayant le même fqdn
#   "certname":"default-citeo-gdtr-cert"  # optionel: si port=443 alors ne crée par de certis mais utilise celui-ci (qui doit être présent dans l'appgw)
#},{...}]


try {
    GetLEData
    Set-PAAccount -Id $PAAccountId
    $WebApps | ForEach-Object {
        Write-Host "* Dealing with"$_.app
        $_.fqdn=$_.fqdn -replace "prod.",""

        $_ | Add-Member -MemberType NoteProperty -Name 'protocol' -Value $($_.port -eq 80 ? "http": "https")

        $httpSettingResult = Make-HttpSettings($_)
        $_ | Add-Member -MemberType NoteProperty -Name 'httpSettingsName' -Value $httpSettingResult

        $probeResult = Make-Probes($_)
        $_ | Add-Member -MemberType NoteProperty -Name 'probeName' -Value $probeResult

        $void = Link-Probe-HttpSettings($_)

        $BEAPoolResult = Make-BackendPool($_)
        $_ | Add-Member -MemberType NoteProperty -Name 'BEAPoolName' -Value $BEAPoolResult

        $bafqdn=$_.webapp+".azurewebsites.net"
        $void = Set-AzApplicationGatewayBackendAddressPool -Name $_.BEAPoolName -ApplicationGateway $gw -BackendFqdns $bafqdn
        $FEPortResults = Make-FEPort($_)
        $_ | Add-Member -MemberType NoteProperty -Name 'feportName' -Value $FEPortResults

        $FEIpResult = Make-FEIpConfig($_)
        $_ | Add-Member -MemberType NoteProperty -Name 'fipName' -Value $FEIpResult

        $listenerResult=Make-Listener($_)
        $_ | Add-Member -MemberType NoteProperty -Name 'listenerName' -Value $listenerResult

        $rulesResult = Make-Rules($_)

        $redirectResult=Make-Redirection($_)

        Write-Host "Commiting app-gateway"
        $void=Set-AzApplicationGateway -ApplicationGateway $gw
    }
} catch {
    Write-Error "/!\ Woops... Y a eu du grabuge !"
    Write-Error $PSItem
    $ec=$LASTEXITCODE
} finally {
    PushLEData
    unlockMe -blob $blob -leaseId $leaseId
}
exit $ec
