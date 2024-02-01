package globals

import "github.com/docker/docker/client"

// DockerClient contains the docker client which is used for the whole tool
var DockerClient *client.Client
