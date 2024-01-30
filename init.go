package main

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/kong/go-kong/kong"
	wisdomType "github.com/wisdom-oss/commonTypes"

	"github.com/joho/godotenv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/wisdom-oss/gateway-service-watcher/globals"
)

var initLogger = log.With().Bool("startup", true).Logger()

// init is executed at every startup of the microservice and is always executed
// before main
func init() {
	// load the variables found in the .env file into the process environment
	err := godotenv.Load()
	if err != nil {
		initLogger.Debug().Msg("no .env files found")
	}

	configureLogger()
	loadServiceConfiguration()
	connectDocker()
	discoverAndConnectGateway()

	initLogger.Info().Msg("initialization process finished")

}

// configureLogger handles the configuration of the logger used in the
// microservice. it reads the logging level from the `LOG_LEVEL` environment
// variable and sets it according to the parsed logging level. if an invalid
// value is supplied or no level is supplied, the service defaults to the
// `INFO` level
func configureLogger() {
	// set the time format to unix timestamps to allow easier machine handling
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	// allow the logger to create an error stack for the logs
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	// now use the environment variable `LOG_LEVEL` to determine the logging
	// level for the microservice.
	rawLoggingLevel, isSet := os.LookupEnv("LOG_LEVEL")

	// if the value is not set, use the info level as default.
	var loggingLevel zerolog.Level
	if !isSet {
		loggingLevel = zerolog.InfoLevel
	} else {
		// now try to parse the value of the raw logging level to a logging
		// level for the zerolog package
		var err error
		loggingLevel, err = zerolog.ParseLevel(rawLoggingLevel)
		if err != nil {
			// since an error occurred while parsing the logging level, use info
			loggingLevel = zerolog.InfoLevel
			initLogger.Warn().Msg("unable to parse value from environment. using info")
		}
	}
	// since now a logging level is set, configure the logger
	zerolog.SetGlobalLevel(loggingLevel)
}

// loadServiceConfiguration handles loading the `environment.json` file which
// describes which environment variables are needed for the service to function
// and what variables are optional and their default values
func loadServiceConfiguration() {
	initLogger.Info().Msg("loading service configuration from environment")
	// now check if the default location for the environment configuration
	// was changed via the `ENV_CONFIG_LOCATION` variable
	location, locationChanged := os.LookupEnv("ENV_CONFIG_LOCATION")
	if !locationChanged {
		// since the location has not changed, set the default value
		location = "./environment.json"
		initLogger.Debug().Msg("location for environment config not changed")
	}
	initLogger.Debug().Str("path", location).Msg("loading environment requirements file")
	var c wisdomType.EnvironmentConfiguration
	err := c.PopulateFromFilePath(location)
	if err != nil {
		initLogger.Fatal().Err(err).Msg("unable to load environment requirements file")
	}
	initLogger.Info().Msg("validating environment variables")
	globals.Environment, err = c.ParseEnvironment()
	if err != nil {
		initLogger.Fatal().Err(err).Msg("environment validation failed")
	}
	initLogger.Info().Msg("loaded service configuration from environment")
}

// connectDocker connects to the provided docker host.
// It creates a new docker client using the environment variables and verifies
// the connection to the docker host.
// If the connection fails, it logs a fatal error.
// If the connection is successful, it logs the API version and the operating
// system type of the docker host.
func connectDocker() {
	initLogger.Info().Msg("connecting to provided docker host")
	var err error
	globals.DockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		initLogger.Fatal().Err(err).Msg("unable to create docker client")
	}
	initLogger.Info().Msg("verifying connection to the docker host")
	info, err := globals.DockerClient.Ping(context.Background())
	if err != nil {
		if client.IsErrConnectionFailed(err) {
			initLogger.Fatal().Err(err).Msg("connection to docker host failed")
		}
		initLogger.Fatal().Err(err).Msg("unable to verify connection to docker host")
	}
	initLogger.Info().Str("dockerApiVersion", info.APIVersion).Str("dockerHostOs", info.OSType).Msg("connected to docker")
}

// discoverAndConnectGateway searches for the API gateway container in the
// Docker host and establishes a connection to it.
// It uses the DockerClient to list containers with a specific label and filters
// based on the "wisdom-oss.gateway" label.
// If no container is found, it logs a fatal error.
// If multiple containers are found, it logs a fatal error.
// It retrieves the first container from the list and checks if it is in the
// "wisdom" network. If not, it logs a fatal error.
// It sets up a connection to the API gateway by obtaining its IP address from
// the "wisdom" network and creates a Kong client.
// If any error occurs during the connection setup, it logs a fatal error.
// Finally, it retrieves the gateway information using the Kong client and logs
// the connected gateway's version.
func discoverAndConnectGateway() {
	initLogger.Info().Msg("discovering api gateway")
	filter := filters.NewArgs()
	filter.Add("label", "wisdom-oss.gateway")

	containers, err := globals.DockerClient.ContainerList(context.Background(), container.ListOptions{Filters: filter})
	if err != nil {
		log.Fatal().Err(err).Msg("unable to list possible gateway containers")
	}
	switch {
	case len(containers) == 0:
		initLogger.Fatal().Msg("no gateway container found on this docker host")
		break
	case len(containers) > 1:
		initLogger.Fatal().Msg("multiple gateway containers found on this docker host. unable to proceed")
		break
	}
	gatewayContainer := containers[0]
	initLogger.Info().Str("containerID", gatewayContainer.ID).Msg("discovered api gateway")
	if _, inWISdoMNetwork := gatewayContainer.NetworkSettings.Networks["wisdom"]; !inWISdoMNetwork {
		initLogger.Fatal().Msg("api gateway not in 'wisdom' network")
	}
	initLogger.Info().Msg("setting up connection to gateway")
	gatewayIP := gatewayContainer.NetworkSettings.Networks["wisdom"].IPAddress
	url := fmt.Sprintf("htp://%s:8001", gatewayIP)
	globals.KongClient, err = kong.NewClient(kong.String(url), nil)
	if err != nil {
		initLogger.Fatal().Err(err).Msg("unable to create gateway admin client")
	}
	gatewayInformation, err := globals.KongClient.Info.Get(context.Background())
	if err != nil {
		initLogger.Fatal().Err(err).Msg("unable to retrieve gateway information")
	}
	initLogger.Info().Str("gatewayVersion", gatewayInformation.Version).Msg("connected to api gateway")
}
