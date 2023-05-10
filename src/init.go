package main

import (
	"context"
	"fmt"
	"gateway-service-watcher/src/global"
	"gateway-service-watcher/src/structs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/kong/go-kong/kong"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/titanous/json5"
	"os"
	"strings"
	"time"
)

/*
This file contains the bootstrapping functions for the gateway-service-watcher
*/

var initCtx = context.Background()

// Initialization: Logger Configuration
func init() {
	// set up the time format and the error logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	wr := diode.NewWriter(os.Stdout, 1000, 10*time.Millisecond, func(missed int) {
		fmt.Printf("Logger Dropped %d messages", missed)
	})

	log.Output(wr)

	// now read the environment variable loglevel
	logLevel, _ := os.LookupEnv("LOG_LEVEL")
	logLevel = strings.ToLower(logLevel)
	switch logLevel {
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
		log.Log().Str("level", logLevel).Str("init-step", "configure-logger").Msg("configured global logger")
		break
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
		log.Log().Str("level", logLevel).Str("initStep", "configure-logger").Msg("configured global logger")
		break
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		log.Log().Str("level", logLevel).Str("initStep", "configure-logger").Msg("configured global logger")
		break
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
		log.Log().Str("level", logLevel).Str("initStep", "configure-logger").Msg("configured global logger")
		break
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		log.Log().Str("level", logLevel).Str("initStep", "configure-logger").Msg("configured global logger")
		break
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Log().Str("level", logLevel).Str("initStep", "configure-logger").Msg("configured global logger")
		break
	case "trace":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
		log.Log().Str("level", logLevel).Str("initStep", "configure-logger").Msg("configured global logger")
		break
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		log.Warn().Str("initStep", "configure-logger").Msg("configured global logger with default level `info`")
		break
	}
}

func init() {
	l := log.With().Str("initStep", "load-environment").Logger()
	// check if the environment variables set a different location for the config
	// file
	l.Info().Msg("loading environment configuration")
	envFileLocation, envSet := os.LookupEnv("ENVIRONMENT_CONFIGURATION")
	var filePath string
	if envSet {
		filePath = envFileLocation
	} else {
		filePath = "/res/environment.json5"
	}
	var environmentConfiguration structs.EnvironmentConfiguration
	configurationContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to read environment configuration")
	}
	err = json5.Unmarshal(configurationContent, &environmentConfiguration)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to unmarshal the environment configuration")
	}
	l.Info().Msg("successfully parsed environment configuration")
	l.Info().Msg("loading required environment variables")
	// now iterate through the required environment variables
	for _, key := range environmentConfiguration.RequiredEnvironmentVariables {
		l.Debug().Str("env", key).Msg("reading required environment variable")
		value, isSet := os.LookupEnv(key)
		if !isSet {
			// since the key was not found look for a docker secret containing the value
			fileKey := key + "_FILE"
			value, isSet := os.LookupEnv(fileKey)
			if !isSet {
				l.Fatal().Msgf(
					"the environment variable '%s' is required but not set and no file present", key)
			} else {
				l.Debug().Str("env", key).Msg("found value for environment variable in docker secret")
				// read the file
				fileContent, err := os.ReadFile(value)
				if err != nil {
					l.Fatal().Err(err).Msgf("unable to read file '%s'", value)
				}
				global.Environment[key] = string(fileContent)
			}

		} else {
			l.Debug().Str("env", key).Msg("found value for environment variable")
			global.Environment[key] = value
		}
	}
	l.Info().Msg("successfully loaded required environment variables")

	// now iterate through the optional environment variables
	for _, optionalEnvironmentVariable := range environmentConfiguration.OptionalEnvironmentVariables {
		l.Debug().Str("env", optionalEnvironmentVariable.EnvironmentKey).Msg("reading optional environment variable")
		value, isSet := os.LookupEnv(optionalEnvironmentVariable.EnvironmentKey)
		if !isSet {
			l.Debug().Str("env", optionalEnvironmentVariable.EnvironmentKey).Msg("environment variable not found")
			l.Info().Str("env", optionalEnvironmentVariable.EnvironmentKey).Msg("using default value")
			global.Environment[optionalEnvironmentVariable.EnvironmentKey] = optionalEnvironmentVariable.DefaultValue
		} else {
			l.Debug().Str("env", optionalEnvironmentVariable.EnvironmentKey).Msg("found value for environment variable")
			global.Environment[optionalEnvironmentVariable.EnvironmentKey] = value
		}
	}

	l.Info().Msg("finished loading of the optional environment variables")
}

// Initialization: Connect to Docker API
func init() {
	log.Info().Msg("connecting to docker api")
	var err error
	global.DockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal().Err(err).Msg("an error occurred while creating the new docker client")
	}
	info, err := global.DockerClient.Info(initCtx)
	if client.IsErrConnectionFailed(err) {
		log.Fatal().Err(err).Msg("failed to connect to the docker api")
	} else if err != nil {
		log.Fatal().Err(err).Msg("a unexpected error occurred while accessing the docker api")
	}
	log.Info().Msg("connected to docker api")
	log.Trace().Interface("docker", info).Msg("docker debug information")
}

// build connection to the api gateway via docker
func init() {
	log.Info().Msg("searching for the api gateway")
	containerFilter := filters.NewArgs()
	containerFilter.Add("label", "wisdom-oss.isGateway")
	gatewayContainers, err := global.DockerClient.ContainerList(initCtx, types.ContainerListOptions{
		Filters: containerFilter,
	})
	if client.IsErrConnectionFailed(err) {
		log.Fatal().Err(err).Msg("failed to connect to the docker api")
	} else if err != nil {
		log.Fatal().Err(err).Msg("an error occurred while searching for the api gateway")
	}
	log.Debug().Int("containerCount", len(gatewayContainers)).Msg("search finished")
	if len(gatewayContainers) == 0 {
		log.Fatal().Msg("no running api gateway found")
	}
	if len(gatewayContainers) > 1 {
		log.Fatal().Msg("multiple api gateways found. this is not supported")
	}
	gateway := gatewayContainers[0]
	log.Info().Str("containerID", gateway.ID).Msg("found api gateway")
	log.Info().Msg("determining hostname of the api gateway")
	gatewayData, err := global.DockerClient.ContainerInspect(initCtx, gateway.ID)
	if client.IsErrConnectionFailed(err) {
		log.Fatal().Err(err).Msg("failed to connect to the docker api")
	} else if err != nil {
		log.Fatal().Err(err).Msg("unexpected error while inspecting gateway")
	}
	kongHost := gatewayData.Config.Hostname
	log.Info().Str("dockerHost", kongHost).Msg("got hostname of the gateway container")
	log.Info().Msg("setting up connection to the api gateway")
	kongURL := fmt.Sprintf("http://%s:8001", kongHost)
	global.KongClient, err = kong.NewClient(kong.String(kongURL), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("an error occurred while creating a new kong client")
	}
	log.Debug().Msg("created kong client. now testing connectivity")
	gatewayInfo, err := global.KongClient.Info.Get(initCtx)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to retrieve information about the gateway")
	}
	log.Trace().Interface("gatewayInformation", *gatewayInfo)
	log.Info().Str("containerID", gateway.ID).Str("kongVersion", gatewayInfo.Version).Msg("connected to the api gateway")
}
