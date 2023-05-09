package global

import (
	"github.com/docker/docker/client"
	"github.com/kong/go-kong/kong"
)

// DockerClient contains the docker client which is used for the whole tool
var DockerClient *client.Client

// KongClient is the client used to connect to the api gateway
var KongClient *kong.Client

// Environment contains the processed environemnt variables
var Environment map[string]string = make(map[string]string)
