package structs

type GatewayConfiguration struct {
	ServiceName        string `json:"serviceName"`
	UpstreamName       string `json:"upstreamName"`
	EnableHealthchecks bool   `json:"enableHealthchecks"`
	ServicePath        string `json:"servicePath"`
}

type EnvironmentConfiguration struct {
	RequiredEnvironmentVariables []string `json:"required"`
	OptionalEnvironmentVariables []struct {
		EnvironmentKey string `json:"key"`
		DefaultValue   string `json:"default"`
	} `json:"optional"`
}
