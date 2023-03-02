package gatewayServiceWatcher

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"os"
	"strings"
)

/*
This file contains the bootstrapping functions for the gateway-service-watcher
*/

// Initialization: Logger Configuration
func init() {
	// set up the time format and the error logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

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
		log.Warn().Str("level", "info").Str("initStep", "configure-logger").Msg("configured global logger with default level `info`")
		break
	}
}
