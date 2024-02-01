package types

// ServiceConfiguration represents the configuration settings for a service.
// It includes the gateway path, port number, and authentication requirement.
type ServiceConfiguration struct {
	GatewayPath           string
	Port                  uint16
	RequireAuthentication bool
}
