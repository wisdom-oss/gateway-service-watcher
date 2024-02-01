package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/kong/go-kong/kong"
	"github.com/rs/zerolog/log"

	"github.com/wisdom-oss/gateway-service-watcher/globals"
)

// dockerNetwork is the network that the containers need to be in to be
// accessible by the api gateway
const dockerNetwork = "wisdom"

func main() {
	log.Info().Msg("starting main watchdog")

	// set up a context which is cancelled as soon as an interrupt signal is
	// received indicating that the watchdog should shutdown
	ctx, stopSignalReceiving := signal.NotifyContext(context.Background(), os.Interrupt)

	// set up a ticker wich is used to periodically initiate a scan of the
	// docker host
	ticker := time.NewTicker(globals.ScanningInterval)

	// construct the needed filters and queries for the docker containers
	serviceContainerFilter := filters.NewArgs()
	// only consider containers marked as microservice containers
	serviceContainerFilter.Add("label", "wisdom.service=true")

	// construct the plugin for the authentication
	authPlugin := &kong.Plugin{
		Name: kong.String("oidc"),
		Config: kong.Configuration{
			"discoveryUri": globals.Environment["OIDC_DISCOVERY_URL"],
			"clientID":     globals.Environment["OIDC_CLIENT_ID"],
		},
	}

	// now start the "endless" loop to allow selecting between the context being
	// done and the ticker going off
	for {
		select {
		// this case is used to detect the shutdown signal which cancelled the
		// context
		case <-ctx.Done():
			log.Info().Msg("received shutdown signal")
			// signal that there is no need to receive more signals on the
			// context
			stopSignalReceiving()
			// stop the ticker to let this be the last action of the software
			ticker.Stop()
			// now close the connection to the docker host
			err := globals.DockerClient.Close()
			if err != nil {
				log.Warn().Err(err).Msg("unable to gracefully close connection to docker host")
			}
			// now exit the watchdog
			os.Exit(0)
		// this case is used to detect the ticker going off signaling that the
		// next scan should be executed
		case <-ticker.C:
			log.Info().Msg("running scan")
			containers, err := globals.DockerClient.ContainerList(ctx, container.ListOptions{Filters: serviceContainerFilter})
			if err != nil {
				// if the context has been cancelled no log is needed since the
				// service just shuts down.
				if !errors.Is(err, context.Canceled) {
					log.Warn().Err(err).Msg("unable to pull container list")
				}
				continue
			}
			log.Debug().Int("containerCount", len(containers)).Msg("received container list")

			// now create a list of the ids used as hostnames to quickly check if the targets currently listed in the
			// gateway are detected as containers
			var hostnames []string
			for _, serviceContainer := range containers {
				hostnames = append(hostnames, serviceContainer.ID)
			}

			for _, serviceContainer := range containers {
				log := log.With().Str("containerID", serviceContainer.ID).Logger()
				// now try to parse a possible container configuration
				configuration, err := buildContainerConfiguration(serviceContainer)
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						log.Error().Err(err).Msg("unable to pull container list. continuing with next container")
					}
					continue
				}

				// create a cleaned name which does not contain any slashes
				cleanedName := strings.ReplaceAll(configuration.GatewayPath, "/", "_")
				log = log.With().Str("gatewayKey", cleanedName).Logger()
				// now check if an upstream with the gateway paths name already
				// exists
				upstream, err := globals.KongClient.Upstreams.Get(ctx, &cleanedName)
				switch {
				case kong.IsNotFoundErr(err):
					log.Info().Msg("creating new upstream for service")
					upstream, err = globals.KongClient.Upstreams.Create(ctx, &kong.Upstream{
						Name: &cleanedName,
					})
					if err != nil {
						log.Error().Err(err).Msg("unable to create new upstream for service. skipping container")
					}
					continue
				case err != nil:
					log.Error().Err(err).Msg("unable to check gateway for existing upstream. skipping container")
					continue
				}

				// now build the target address for the container
				targetAddress := fmt.Sprintf("%s:%d", serviceContainer.ID, configuration.Port)

				// now get all targets that are listed for the upstream and
				// check if the container is among them
				targets, err := globals.KongClient.Targets.ListAll(ctx, upstream.ID)
				if err != nil {
					log.Error().Err(err).Msg("unable to retrieve upstream targets from gateway")
					continue
				}
				targetAlreadyPresent := false
				var missingTargets []*string
				for _, target := range targets {
					addressParts := strings.Split(*target.Target, ":")
					if !slices.Contains(hostnames, addressParts[1]) {
						missingTargets = append(missingTargets, target.ID)
					}
					if *target.Target == targetAddress {
						targetAlreadyPresent = true
						break
					}
				}
				if !targetAlreadyPresent {
					_, err := globals.KongClient.Targets.Create(ctx, upstream.ID, &kong.Target{
						Target: &targetAddress,
					})
					if err != nil {
						log.Error().Err(err).Msg("unable add container to upstream targets")
						continue
					}
				}
				for _, missingTarget := range missingTargets {
					err := globals.KongClient.Targets.Delete(ctx, upstream.ID, missingTarget)
					if err != nil {
						log.Error().Str("targetID", *missingTarget).Err(err).Msg("unable to remove outdated target")
					}
				}

				// now check if a service already exists for this microservice
				service, err := globals.KongClient.Services.Get(ctx, &cleanedName)
				switch {
				case kong.IsNotFoundErr(err):
					log.Info().Msg("creating new service")
					service, err = globals.KongClient.Services.Create(ctx, &kong.Service{
						Host: upstream.Name,
						Name: &cleanedName,
					})
					if err != nil {
						log.Error().Err(err).Msg("unable to create new service")
						continue
					}
				}

				// now check if the authentication plugin needs to be configured
				// on the service
				if configuration.RequireAuthentication {
					// check if the plugin is already configured for the service
					plugins, err := globals.KongClient.Plugins.ListAllForService(ctx, service.ID)
					if err != nil {
						log.Error().Err(err).Msg("unable to retrieve plugin list for service")
					}
					authConfigured := false
					var plugin *kong.Plugin
					for _, plugin = range plugins {
						if plugin.Name == authPlugin.Name {
							authConfigured = true
							break
						}
					}

					// now check if the authentication is configured and needs an
					// update
					if authConfigured &&
						plugin.Config["discoveryUri"] != authPlugin.Config["discoveryUri"] ||
						plugin.Config["clientID"] != authPlugin.Config["clientID"] {
						_, err := globals.KongClient.Plugins.UpdateForService(ctx, service.ID, authPlugin)
						if err != nil {
							log.Error().Err(err).Msg("unable to update oidc plugin for service")
							continue
						}
					}
					if !authConfigured {
						_, err := globals.KongClient.Plugins.CreateForService(ctx, service.ID, authPlugin)
						if err != nil {
							log.Error().Err(err).Msg("unable to create oidc plugin for service")
							continue
						}
					}
				}

				// now check that the service is using the correct upstream as
				// its host
				if service.Host != upstream.Name {
					log.Warn().Msg("service not using correct upstream. reconfiguring service")
					service.Host = upstream.Name
					service, err = globals.KongClient.Services.Update(ctx, service)
					if err != nil {
						log.Error().Err(err).Msg("unable to update service")
						continue
					}
				}

				_, err = globals.KongClient.Routes.Get(ctx, &cleanedName)
				switch {
				case kong.IsNotFoundErr(err):
					_, err = globals.KongClient.Routes.Create(ctx, &kong.Route{
						Name:              &cleanedName,
						Paths:             []*string{&configuration.GatewayPath},
						Service:           service,
						RequestBuffering:  kong.Bool(false),
						ResponseBuffering: kong.Bool(false),
					})
					if err != nil {
						log.Error().Err(err).Msg("unable to create new route for service")
						continue
					}
				}

			}
		}
	}
}
