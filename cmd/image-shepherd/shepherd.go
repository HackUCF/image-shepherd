package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/s-newman/image-shepherd/internal/client"
	"github.com/s-newman/image-shepherd/internal/config"
	"github.com/s-newman/image-shepherd/pkg/shepherd"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var configFile = flag.String("config", "images.yaml", "Path to the images configuration file")
var cloudName = flag.String("os-cloud", "openstack", "Name of the cloud to use in clouds.yaml")
var noColor = flag.Bool("no-color", false, "Don't colorize logging output")
var verbose = flag.Bool("verbose", false, "Include extra information in each log line")

func initLogging() {
	z := zap.NewDevelopmentConfig()

	z.DisableStacktrace = true

	if !*noColor {
		z.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	if *verbose {
		z.Level.SetLevel(zapcore.InfoLevel)
	} else {
		z.Level.SetLevel(zapcore.WarnLevel)
		z.DisableCaller = true
		z.EncoderConfig.CallerKey = ""
		z.EncoderConfig.TimeKey = ""
	}

	logger, err := z.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %s", err)
		os.Exit(1)
	}
	zap.ReplaceGlobals(logger)
	defer logger.Sync() //nolint:errcheck
}

func main() {
	flag.Parse()
	initLogging()

	c := config.Load(*configFile)

	shepherd.Run(client.New(*cloudName), c.Images)
}
