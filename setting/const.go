package setting

import "github.com/rancher/per-host-subnet/routeupdate/hostgw"

const (
	MetadataURL            = "http://%s/2016-07-29"
	DefaultMetadataAddress = "169.254.169.250"
)

const (
	DefaultRouteUpdateProvider = hostgw.ProviderName

	DefaultDisableHostNATIPset = "RANCHER_DISABLE_HOST_NAT_IPSET"
)
