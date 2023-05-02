package structs

type GatewayConfiguration struct {
	ServiceName        string `json:"serviceName"`
	UpstreamName       string `json:"upstreamName"`
	EnableHealthchecks bool   `json:"enableHealthchecks"`
	ServicePath        string `json:"servicePath"`
}
