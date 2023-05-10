package main

import (
	"context"
	"gateway-service-watcher/src/global"
	"gateway-service-watcher/src/structs"
	"gateway-service-watcher/src/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/kong/go-kong/kong"
	"github.com/rs/zerolog/log"
	"strconv"
	"time"
)

func main() {
	log.Log().Msg("starting service watcher")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// create a context for the main watcher
	ctx := context.Background()

	// initializing the filters for the docker containers
	serviceContainerFilter := filters.NewArgs()
	serviceContainerFilter.Add("label", "wisdom-oss.isService")

	// check if the authorization plugin is enabled
	plugins, _ := global.KongClient.Plugins.ListAll(ctx)
	authEnabled := false
	for _, plugin := range plugins {
		if *plugin.Name == "kong-internal-db-auth" && plugin.Service == nil && plugin.Route == nil {
			authEnabled = true
			break
		}
	}

	if !authEnabled {
		_, err := global.KongClient.Plugins.Create(ctx, &kong.Plugin{
			Name: kong.String("kong-internal-db-auth"),
			Config: kong.Configuration{
				"intospection_url": global.Environment["INTROSPECTION_URL"],
				"auth_header":      "ignore",
			},
			Enabled: kong.Bool(true),
		})
		if err != nil {
			log.Warn().Err(err).Msg("unable to enable global authentication. services may be unprotected")
		}
	}

	for {
		select {
		case <-ticker.C:
			log.Info().Msg("looking for docker containers from wisdom-project")
			possibleServiceContainers, err := global.DockerClient.ContainerList(ctx, types.ContainerListOptions{
				All:     true,
				Filters: serviceContainerFilter,
			})
			if err != nil {
				log.Error().Err(err).Msg("unable to look for containers")
				break
			}
			log.Info().Msg("search finished")
			if len(possibleServiceContainers) == 0 {
				log.Warn().Msg("no containers found")
				break
			}
			log.Info().Int("containers", len(possibleServiceContainers)).Msg("building registration information")
			for _, container := range possibleServiceContainers {
				log := log.With().Str("containerID", container.ID).Logger()
				ctx = context.WithValue(ctx, "logger", log)
				// inspect the container to gather hostnames and ip addresses
				containerInformation, err := global.DockerClient.ContainerInspect(ctx, container.ID)
				if err != nil {
					log.Error().Err(err).Msg("unable to inspect container")
					break
				}
				log.Debug().Str("containerID", container.ID).Msg("checking container for labels")
				isService, err := strconv.ParseBool(containerInformation.Config.Labels["wisdom-oss.isService"])
				if err != nil {
					log.Warn().Msg("unable to convert label value to bool")
					log.Info().Msg("skipping container")
					continue
				}
				if !isService {
					log.Info().Msg("container not marked as service. skipping container")
					continue
				}
				// now parse the service configuration
				var gatewayConfig structs.GatewayConfiguration
				if err != nil {
					log.Warn().Err(err).Msg("looking for labels on container")
					if !utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.name") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.upstream-name") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.path") {
						log.Warn().Msg("labels missing for complete configuration. skipping container")
						continue
					}
				}

				// set the parameters from the container labels
				gatewayConfig.ServiceName = containerInformation.Config.Labels["wisdom-oss.service.name"]
				gatewayConfig.UpstreamName = containerInformation.Config.Labels["wisdom-oss.service.upstream-name"]
				gatewayConfig.ServicePath = containerInformation.Config.Labels["wisdom-oss.service.path"]

				if containerInformation.State.Status == "running" {
					utils.RegisterContainer(ctx, gatewayConfig, containerInformation)
				} else {
					utils.RemoveContainer(ctx, gatewayConfig, containerInformation)
				}

			}
		}
	}
}
