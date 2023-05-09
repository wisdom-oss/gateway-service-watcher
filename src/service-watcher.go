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
		if plugin.Name == kong.String("kong-internal-db-auth") {
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
				hostname := containerInformation.NetworkSettings.IPAddress
				containerUrl := fmt.Sprintf("%s:8000", hostname)
				log.Info().Str("containerUrl", containerUrl).Msg("constructed container reachability")
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

				// now check if any labels are set on the container which override the configuration response
				if utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.name") {
					log.Warn().Msg("overriding service name from config response")
					gatewayConfig.ServiceName = containerInformation.Config.Labels["wisdom-oss.service.name"]
				}
				if utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.upstream-name") {
					log.Warn().Msg("overriding upstream name from config response")
					gatewayConfig.UpstreamName = containerInformation.Config.Labels["wisdom-oss.service.upstream-name"]
				}
				if utils.MapHasKey(containerInformation.Config.Labels, "wisdom-oss.service.path") {
					log.Warn().Msg("overriding service path from config response")
					gatewayConfig.ServicePath = containerInformation.Config.Labels["wisdom-oss.service.path"]
				}

				// now check if the upstream exists in the gateway
				upstream, err := global.KongClient.Upstreams.Get(ctx, &gatewayConfig.UpstreamName)
				if kong.IsNotFoundErr(err) {
					log.Warn().Msg("upstream not found. creating new one")
					upstream, err = global.KongClient.Upstreams.Create(ctx, &kong.Upstream{Name: &gatewayConfig.UpstreamName})
					if err != nil {
						log.Err(err).Msg("unable to create new upstream for service. skipping container")
						continue
					}
					log.Info().Str("upstreamID", *upstream.ID).Str("upstreamName", *upstream.Name).Msg("created new upstream")
				} else if err != nil {
					log.Error().Err(err).Msg("an error occurred while getting the upstream from the gateway")
				}

				// now check if the container is listest in the upstream
				targets, _, err := global.KongClient.Targets.List(ctx, upstream.ID, nil)
				containerInTargetList := false
				for _, t := range targets {
					if t.Target == kong.String(containerUrl) {
						containerInTargetList = true
						break
					}
				}
				if !containerInTargetList {
					log.Warn().Msg("container not listed in desired upstream. adding container to upstream targets")
					target, err := global.KongClient.Targets.Create(ctx, upstream.ID, &kong.Target{
						Target: kong.String(containerUrl),
					})
					if err != nil {
						log.Err(err).Msg("unable to create new upstream target. skipping container")
						continue
					}
					log.Info().Str("targetID", *target.ID).Msg("successfully added upstream target")
				}

				// now check if a service already exists
				service, err := global.KongClient.Services.Get(ctx, &gatewayConfig.ServiceName)
				if kong.IsNotFoundErr(err) {
					log.Warn().Msg("service not found in gateway. creating new service")
					service, err = global.KongClient.Services.Create(ctx, &kong.Service{
						Host: upstream.Name,
						Name: &gatewayConfig.ServiceName,
					})
					if err != nil {
						log.Error().Err(err).Msg("unable to create new service in gateway. skipping container")
						continue
					}
				} else if err != nil {
					log.Error().Err(err).Msg("unable to find service in gateway. skipping container")
					continue
				}

				// now check if the service has the correct upstream set as target
				if service.Host != upstream.Name {
					log.Warn().Msg("service was found, but is not configured to use the set upstream. reconfiguring")
					service.Host = upstream.Name
					_, err := global.KongClient.Services.Update(ctx, service)
					if err != nil {
						log.Error().Msg("unable to update service in gateway. container may not be reachable. " +
							"skipping container")
						continue
					}
				}

				// now check if a route exists for the service
				routes, err := global.KongClient.Routes.ListAll(ctx)
				routeConfigured := false
				for _, route := range routes {
					if utils.ArrayContains(route.Paths, &gatewayConfig.ServicePath) &&
						route.Service.ID == service.ID {
						routeConfigured = true
					}
				}
				if !routeConfigured {
					log.Warn().Msg("no route found matching the service id and the desired path. creating new route")
					_, err := global.KongClient.Routes.Create(ctx, &kong.Route{
						Paths:   []*string{&gatewayConfig.ServicePath},
						Service: service,
					})
					if err != nil {
						log.Error().Err(err).Msg("unable to create new route. skipping container")
					}
				}

			}
		}
	}
}
