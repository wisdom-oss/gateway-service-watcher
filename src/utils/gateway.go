package utils

import (
	"context"
	"fmt"
	"gateway-service-watcher/src/global"
	"gateway-service-watcher/src/structs"
	"github.com/docker/docker/api/types"
	"github.com/kong/go-kong/kong"
	"github.com/rs/zerolog"
)

func RegisterContainer(ctx context.Context, config structs.GatewayConfiguration, container types.ContainerJSON) error {
	log := ctx.Value("logger").(zerolog.Logger)
	// build the containers address
	targetAddress := fmt.Sprintf("%s:8000", container.Config.Hostname)
	// now check if the upstream exists in the gateway
	upstream, err := global.KongClient.Upstreams.Get(ctx, &config.UpstreamName)
	if kong.IsNotFoundErr(err) {
		log.Warn().Msg("upstream not found. creating new one")
		upstream, err = global.KongClient.Upstreams.Create(ctx, &kong.Upstream{Name: &config.UpstreamName})
		if err != nil {
			log.Err(err).Msg("unable to create new upstream for service.")
			return err
		}
		log.Info().Str("upstreamID", *upstream.ID).Str("upstreamName", *upstream.Name).Msg("created new upstream")
	} else if err != nil {
		log.Error().Err(err).Msg("an error occurred while getting the upstream from the gateway")
	}

	// now check if the container is listest in the upstream
	targets, _, err := global.KongClient.Targets.List(ctx, upstream.ID, nil)
	containerInTargetList := false
	for _, t := range targets {
		if *t.Target == targetAddress {
			containerInTargetList = true
			break
		}
	}
	if !containerInTargetList {
		log.Warn().Msg("container not listed in desired upstream. adding container to upstream targets")
		target, err := global.KongClient.Targets.Create(ctx, upstream.ID, &kong.Target{
			Target: kong.String(targetAddress),
		})
		if err != nil {
			log.Err(err).Msg("unable to create new upstream target. skipping container")
			return err
		}
		log.Info().Str("targetID", *target.ID).Msg("successfully added upstream target")
	}

	// now check if a service already exists
	service, err := global.KongClient.Services.Get(ctx, &config.ServiceName)
	if kong.IsNotFoundErr(err) {
		log.Warn().Msg("service not found in gateway. creating new service")
		service, err = global.KongClient.Services.Create(ctx, &kong.Service{
			Host: upstream.Name,
			Name: &config.ServiceName,
		})
		if err != nil {
			log.Error().Err(err).Msg("unable to create new service in gateway. skipping container")
			return err
		}
	} else if err != nil {
		log.Error().Err(err).Msg("unable to find service in gateway. skipping container")
		return nil
	}

	// now check if the service has the correct upstream set as target
	if service.Host != upstream.Name {
		log.Warn().Msg("service was found, but is not configured to use the set upstream. reconfiguring")
		service.Host = upstream.Name
		_, err := global.KongClient.Services.Update(ctx, service)
		if err != nil {
			log.Error().Msg("unable to update service in gateway. container may not be reachable. " +
				"skipping container")
			return err
		}
	}

	// now check if a route exists for the service
	routes, err := global.KongClient.Routes.ListAll(ctx)
	routeConfigured := false
	for _, route := range routes {
		if ArrayContains(route.Paths, &config.ServicePath) &&
			route.Service.ID == service.ID {
			routeConfigured = true
		}
	}
	if !routeConfigured {
		log.Warn().Msg("no route found matching the service id and the desired path. creating new route")
		_, err := global.KongClient.Routes.Create(ctx, &kong.Route{
			Paths:   []*string{&config.ServicePath},
			Service: service,
		})
		if err != nil {
			log.Error().Err(err).Msg("unable to create new route. skipping container")
		}
	}
	return nil

}

func RemoveContainer(ctx context.Context, config structs.GatewayConfiguration, container types.ContainerJSON) error {
	log := ctx.Value("logger").(zerolog.Logger)
	log.Info().Msg("removing not running container from the configured upstream")
	targetAddress := fmt.Sprintf("%s:8000", container.Config.Hostname)
	log.Info().Str("host", targetAddress).Msg("built target address")
	// now get all targets from the kong client
	targets, err := global.KongClient.Targets.ListAll(ctx, &config.UpstreamName)
	if err != nil {
		log.Warn().Err(err).Msg("an error occurred while removing the container")
		return err
	}
	for _, target := range targets {
		if *target.Target == targetAddress {
			log.Debug().Msg("found target which shall be removed")
			err := global.KongClient.Targets.Delete(ctx, &config.UpstreamName, target.ID)
			if err != nil {
				log.Error().Err(err).Msg("unable to remove the container from it's upstream. requests may fail")
				return err
			}
			break
		}
	}
	return nil

}
