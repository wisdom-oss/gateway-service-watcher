package main

import (
	"context"
	"fmt"
	"gateway-service-watcher/src/global"
	"gateway-service-watcher/src/structs"
	"gateway-service-watcher/src/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/kong/go-kong/kong"
	"github.com/rs/zerolog/log"
	"github.com/titanous/json5"
	"net/http"
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

	for {
		select {
		case <-ticker.C:
			log.Info().Msg("looking for docker containers from wisdom-project")
			possibleServiceContainers, err := global.DockerClient.ContainerList(ctx, types.ContainerListOptions{
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
				// since the container is a service, now request the information from the container related to the configuration
				// of it

				// now get the hostname of the container and request the information point on port 8000
				hostname := containerInformation.Config.Hostname
				containerUrl := fmt.Sprintf("http://%s:8000/_gatewayConfig", hostname)
				configResponse, err := http.Get(containerUrl)
				if err != nil {
					log.Error().Err(err).Msg("unable to get the config response from the service. looking for labels")
					if !utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.name") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.upstream-name") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.enable-healthchecks") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.path") {
						log.Warn().Msg("labels for manual configuration are missing. skipping container")
						continue
					}
				}
				if configResponse.StatusCode != 200 {
					log.Warn().Msg("unable to get the config response from the service. looking for labels")
					if !utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.name") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.upstream-name") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.enable-healthchecks") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.path") {
						log.Warn().Msg("labels for manual configuration are missing. skipping container")
						continue
					}
				}
				// now parse the service configuration
				var gatewayConfig structs.GatewayConfiguration
				err = json5.NewDecoder(configResponse.Body).Decode(&gatewayConfig)

				if err != nil {
					log.Warn().Err(err).Msg("unable to parse the config response from the service. looking for labels")
					if !utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.name") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.upstream-name") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.enable-healthchecks") ||
						!utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.path") {
						log.Warn().Msg("labels for manual configuration are missing. skipping container")
						continue
					}
				}

				// now check if any labels are set on the container which override the configuration response
				if utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.name") {
					log.Warn().Msg("overriding service name from config response")
					gatewayConfig.ServiceName = containerInformation.Config.Labels["wisdom-oss.service.name"]
				}
				if utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.upstream-name") {
					log.Warn().Msg("overriding upstream name from config response")
					gatewayConfig.UpstreamName = containerInformation.Config.Labels["wisdom-oss.service.upstream-name"]
				}
				if utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.enable-healthchecks") {
					log.Warn().Msg("overriding healthcheck value from config response")
					gatewayConfig.EnableHealthchecks, err = strconv.ParseBool(containerInformation.Config.Labels["wisdom-oss.service.enable-healthchecks"])
					if err != nil {
						log.Warn().Msg("unable to convert label value to bool. using default value of `true`")
						gatewayConfig.EnableHealthchecks = true
					}
				}
				if utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.path") {
					log.Warn().Msg("overriding service path from config response")
					gatewayConfig.ServicePath = containerInformation.Config.Labels["wisdom-oss.service.path"]
				}

				// now check if the upstream exists in the gateway
				_, err = global.KongClient.Upstreams.Get(ctx, &gatewayConfig.UpstreamName)
				if kong.IsNotFoundErr(err) {
					log.Warn().Msg("upstream not found. creating new one")
					continue
				}
				if err != nil {
					log.Error().Err(err).Msg("an error occurred while getting the upstream from the gateway")
				}

			}
		}
	}
}