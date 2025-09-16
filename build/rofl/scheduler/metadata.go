package scheduler

const (
	// MetadataKeyError is the name of the metadata key that stores the error message.
	MetadataKeyError = "net.oasis.error"
	// MetadataKeySchedulerRAK is the name of the metadata key that stores the scheduler RAK.
	MetadataKeySchedulerRAK = "net.oasis.scheduler.rak"
	// MetadataKeyTLSPk is the name of the metadata key that stores the TLS public key.
	MetadataKeyTLSPk = "net.oasis.tls.pk"
	// MetadataKeySchedulerAPI is the name of the metadata key that stores the API endpoint address.
	MetadataKeySchedulerAPI = "net.oasis.scheduler.api"
	// MetadataKeyProxyDomain is the name of the metadata key that stores the proxy domain.
	MetadataKeyProxyDomain = "net.oasis.proxy.domain"
	// MetadataKeyPermissions is the name of the deployment metadata key that stores the machine
	// permissions.
	MetadataKeyPermissions = "net.oasis.scheduler.permissions"
	// MetadataKeyORCReference is the name of the deployment metadata key that stores the ORC
	// reference.
	MetadataKeyORCReference = "net.oasis.deployment.orc.ref"
	// MetadataKeyProxyCustomDomains is the name of the metadata key that stores the proxy custom domains.
	MetadataKeyProxyCustomDomains = "net.oasis.proxy.custom_domains"
)
