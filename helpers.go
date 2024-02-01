package main

import (
	"fmt"
	"strconv"
	"strings"

	dockerTypes "github.com/docker/docker/api/types"

	"github.com/wisdom-oss/gateway-service-watcher/types"
)

func buildContainerConfiguration(container dockerTypes.Container) (serviceConfiguration *types.ServiceConfiguration, err error) {
	// create the default configuration
	serviceConfiguration = &types.ServiceConfiguration{
		Port:                  8080,
		RequireAuthentication: true,
	}

	// since the container is a microservice check if the gateway path has been
	// set
	path, pathSet := container.Labels["wisdom.service.gateway-path"]
	if !pathSet {
		return nil, errGatewayPathUnset
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errGatewayPathEmpty
	}

	// now set the path
	serviceConfiguration.GatewayPath = path

	// now check for the optional labels and overwrite the default values if
	// needed
	portStr, portOverwritten := container.Labels["wisdom.service.port"]
	if portOverwritten {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errPortParseFail, err)
		}
		serviceConfiguration.Port = uint16(port)
	}

	authStr, authOverwritten := container.Labels["wisdom.service.use-auth"]
	if authOverwritten {
		requireAuth, err := strconv.ParseBool(authStr)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errAuthParseFail, err)
		}
		serviceConfiguration.RequireAuthentication = requireAuth
	}

	return serviceConfiguration, nil
}
