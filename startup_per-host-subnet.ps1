## rancher-tools.psm1 will be exsit in c:\program files\rancher and same path as this script
## rancher-tools.psm1 is matained in rancher/rancher
Import-Module -Name ./rancher-tools.psm1 -Verbose

$SubnetKey="io.rancher.network.per_host_subnet.subnet"
$routerIpKey="io.rancher.network.per_host_subnet.router_ip"
$hypervAdapterMark="Hyper-V Virtual Ethernet Adapter*"
$LoopbackAdapterMark="Microsoft KM-TEST Loopback Adapter"
$defaultNetworkName="transparent"
$networkDriverName="transparent"
function Test-TransparentNetwork([string]$Subnet) {
    
    $output=docker network inspect $defaultNetworkName 2>$null
    if ("$output" -eq "[]"){
        return $false
    }
    $Network= ConvertFrom-Json "$output"
    if("$Subnet" -ne $network.IPAM.Config.subnet){
        return $false
    }
    return $true
}
function New-TransparentNetwork([string]$subnet,[string]$interfaceName) {
    $gateway,$subnetip,$MaskLength = ConvertTo-GatewayFromCidr $subnet
    if(("$gateway" -eq "") -or ("$subnet" -eq "")){
        throw "subnet $subnet is not valid"
    }
    if("$interfaceName" -eq ""){
        throw "Adatper Name $AdapterName is not valid"
    }
    $interfaceMAC=(get-netadapter -Name "$interfaceName").MacAddress
    $uuid=docker network create -d $networkDriverName --subnet $subnet --gateway $gateway -o com.docker.network.windowsshim.interface="$interfaceName" -o com.docker.network.windowsshim.dnsservers="$dnsservers" $defaultNetworkName 2>$null
    if(-not $(Test-TransparentNetwork $subnet)){
        throw "generate network error, docker network create faild"
    }
    $newIndex=(get-netadapter |Where-Object {$_.macaddress -eq "$interfaceMAC"} |Sort-Object -Property ifindex -Descending|Select-Object -First 1 ).ifIndex
    $add=New-NetIpAddress -ifIndex $newIndex -ipaddress $gateway -prefixLength $MaskLength
    return $newIndex
}
function Remove-RancherNetwork{

    $body=docker network inspect $defaultNetworkName 2>$null
    if("$body" -eq "[]"){
        return
    }
    $output=$(docker network rm $defaultNetworkName 2>$null)
    if("$output" -ne "$defaultNetworkName"){
        throw "remove $defaultNetworkName fail"
    }
}

# return gateway subnet maskLength
function ConvertTo-GatewayFromCidr  {
    param(
        [string]$cidr
    )
    process{
    $strs=$cidr.Split("/")
    $out=new-object net.ipaddress(0)
    if(-not [System.net.ipaddress]::TryParse($strs[0],[ref] $out)){
        return "",""
    }
    $mask = ConvertTo-Mask $strs[1] | ConvertTo-DecimalIP
    $Dsubnet=(ConvertTo-DecimalIP $out) -band $mask
    $Dgateway=$Dsubnet+1
    $gateway=ConvertTo-DottedDecimalIP $Dgateway
    $subnet= ConvertTo-DottedDecimalIP $Dsubnet
    if(($(ConvertTo-DecimalIP $gateway) -band $mask) -ne $Dsubnet){
        return ""
    }
    return $gateway,$subnet,$([Convert]::ToInt32($strs[1]))
    }
}

function ConvertTo-Mask {
<#
    .Synopsis
    Returns a dotted decimal subnet mask from a mask length.
    .Description
    ConvertTo-Mask returns a subnet mask in dotted decimal format from an integer value ranging 
    between 0 and 32. ConvertTo-Mask first creates a binary string from the length, converts 
    that to an unsigned 32-bit integer then calls ConvertTo-DottedDecimalIP to complete the operation.
    .Parameter MaskLength
    The number of bits which must be masked.
#>

param(
    [Parameter(Mandatory = $true, Position = 0, ValueFromPipeline = $true)]
    [Alias("Length")]
    [ValidateRange(0, 32)]
    $MaskLength
)

Process {
    return ConvertTo-DottedDecimalIP ([Convert]::ToUInt32($(("1" * $MaskLength).PadRight(32, "0")), 2)).ToString()
}
}
function ConvertTo-DottedDecimalIP {
<#
    .Synopsis
    Returns a dotted decimal IP address from either an unsigned 32-bit integer or a dotted binary string.
    .Description
    ConvertTo-DottedDecimalIP uses a regular expression match on the input string to convert to an IP address.
    .Parameter IPAddress
    A string representation of an IP address from either UInt32 or dotted binary.
#>

    param(
        [Parameter(Mandatory = $true, Position = 0, ValueFromPipeline = $true)]
        [String]$IPAddress
    )

    process {
        Switch -RegEx ($IPAddress) {
        "([01]{8}.){3}[01]{8}" {
            return [String]::Join('.', $( $IPAddress.Split('.') | ForEach-Object { [Convert]::ToUInt32($_, 2) } ))
        }
        "\d" {
            $IPAddress = [UInt32]$IPAddress
            $DottedIP = $( For ($i = 3; $i -gt -1; $i--) {
            $Remainder = $IPAddress % [Math]::Pow(256, $i)
            ($IPAddress - $Remainder) / [Math]::Pow(256, $i)
            $IPAddress = $Remainder
            } )
            
            return [String]::Join('.', $DottedIP)
        }
        default {
            Write-Error "Cannot convert this format"
        }
        }
    }
}

function ConvertTo-DecimalIP {
  <#
    .Synopsis
      Converts a Decimal IP address into a 32-bit unsigned integer.
    .Description
      ConvertTo-DecimalIP takes a decimal IP, uses a shift-like operation on each octet and returns a single UInt32 value.
    .Parameter IPAddress
      An IP Address to convert.
  #>
  
  [CmdLetBinding()]
  param(
    [Parameter(Mandatory = $true, Position = 0, ValueFromPipeline = $true)]
    [Net.IPAddress]$IPAddress
  )

  process {
    $i = 3; $DecimalIP = 0;
    $IPAddress.GetAddressBytes() | ForEach-Object { $DecimalIP += $_ * [Math]::Pow(256, $i); $i-- }

    return [UInt32]$DecimalIP
  }
}
function ConvertTo-DecimalIP {
  <#
    .Synopsis
      Converts a Decimal IP address into a 32-bit unsigned integer.
    .Description
      ConvertTo-DecimalIP takes a decimal IP, uses a shift-like operation on each octet and returns a single UInt32 value.
    .Parameter IPAddress
      An IP Address to convert.
  #>
  
  param(
    [Parameter(Mandatory = $true, Position = 0, ValueFromPipeline = $true)]
    [Net.IPAddress]$IPAddress
  )
 
  process {
    $i = 3; $DecimalIP = 0;
    $IPAddress.GetAddressBytes() | ForEach-Object { $DecimalIP += $_ * [Math]::Pow(256, $i); $i-- }
 
    return [UInt32]$DecimalIP
  }
}

function RestartRRAS  {
    $serviceStartType=$(get-service remoteaccess).StartType
    if("$serviceStartType" -ne "Automatic"){
        set-service remoteaccess -StartupType Automatic
    }
    restart-service remoteaccess
    Start-Sleep 5
}
function SetMetadataRoute  {
    param(
        [uint32]$ifIndex
    )
    process{
        <#When we deploy it into aws, default 169.254.169.250 route needs to be remove and reset.#>
        $curr=get-netroute "169.254.169.250/32" -ErrorAction Ignore 
        if($curr -ne $null){
            $curr | remove-netroute -Confirm:$false
        }
        if($ifIndex -eq 0){
            return
        }
        $route=new-netroute "169.254.169.250/32" -ifIndex $ifIndex -nextHop 0.0.0.0
    }
}

function SetupRRASNat{
    process{
        $_=$(netsh routing ip nat install)
        foreach($adapter in $natAdapters){
            $_=$(netsh routing ip nat add interface "$($adapter.Name)" full  2>$err)
            if($err -ne $null){
                throw "$err"
            }
        }
    }
}

function installVirtualNic  {
    $_=$(& "c:\program files\rancher\devcon.exe" -r install $env:windir\Inf\Netloop.inf *MSLOOP)
    sleep 10
    return (Get-NetAdapter | Where-Object {$_.InterfaceDescription -eq "$LoopbackAdapterMark"}).Name
}

function Get-NeededNatAdapters{
    $routerip= Get-RancherLabel $routerIpKey
    $routeripadd=Get-NetIPAddress -IPAddress "$routerip" -ErrorAction Ignore 
    if($routeripadd -eq $null){
        throw "$routerip not found"
    }
    return $(Get-netadapter |where-object {$_.InterfaceDescription -NotLike "$hypervAdapterMark" -and $_.ifIndex -ne $routeripadd.InterfaceIndex})
}

function Get-NatDNSServers ($natAdapters) {
    $dnsservers=@{}
    foreach ($adapter in $natAdapters) {
        $dservers=($adapter | Get-DnsClientServerAddress -AddressFamily IPv4).ServerAddresses
        foreach ($ds in $dservers) {
            $dnsservers["$ds"]=$true
        }
    }
    $rtn= ($dnsservers.Keys -join ",")
    return $rtn
}

try{
    $natAdapters=Get-NeededNatAdapters
    $dnsservers=Get-NatDNSServers $natAdapters
    $subnet=Get-RancherLabel -Key $SubnetKey 
    $ifIndex=0
    if("$subnet" -ne ""){
        $adapterName=installVirtualNic
        if(-not $(Test-TransparentNetwork -Subnet $subnet)){
            Remove-RancherNetwork
            $ifIndex= New-TransparentNetwork $subnet $adapterName
        } else {

        }
    } else {
        $ifIndex= New-TransparentNetwork $subnet $adapterName
    }
    $_=(netsh advfirewall set allprofile state off)
    RestartRRAS
    SetupRRASNat
    SetMetadataRoute $ifIndex
    $service=get-service rancher-per-host-subnet -ErrorAction Ignore
    if($service -ne $null){
        & 'C:\Program Files\rancher\per-host-subnet.exe' --unregister-service
    }
    & 'C:\Program Files\rancher\per-host-subnet.exe' --register-service
}
catch {
    Throw $Error[0]
}