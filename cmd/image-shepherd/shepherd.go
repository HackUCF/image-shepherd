package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/HackUCF/image-shepherd/internal/client"
	"github.com/HackUCF/image-shepherd/internal/config"
	"github.com/HackUCF/image-shepherd/pkg/shepherd"
	"github.com/gophercloud/gophercloud/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var configFile = flag.String("config", "images.yaml", "Path to the images configuration file")
var cloudName = flag.String("os-cloud", "openstack", "Name of the cloud to use in clouds.yaml")
var noColor = flag.Bool("no-color", false, "Don't colorize logging output")
var verbose = flag.Bool("verbose", false, "Include extra information in each log line")
var ownerProjectID = flag.String("owner-project-id", "", "Project ID owner that matched current image must have")
var requireProtected = flag.Bool("require-protected", false, "Require matched current image to be protected")
var requirePublic = flag.Bool("require-public", false, "Require matched current image to be public")
var uploadTimeout = flag.Int("upload-timeout", 600, "Timeout for image upload in seconds")
var downloadTimeout = flag.Int("download-timeout", 600, "Timeout for image download in seconds")

func initLogging() {
	z := zap.NewDevelopmentConfig()

	z.DisableStacktrace = true

	if !*noColor {
		z.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	if *verbose {
		z.Level.SetLevel(zapcore.DebugLevel)
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

	// Early startup message so users see something even at default (warn) log level
	fmt.Printf("Starting image-shepherd\n  config: %s\n  cloud: %s\n  verbose: %t\n  no-color: %t\n  owner-project-id: %s\n  require-protected: %t\n  require-public: %t\n  upload-timeout: %ds\n  download-timeout: %ds\n", *configFile, *cloudName, *verbose, *noColor, *ownerProjectID, *requireProtected, *requirePublic, *uploadTimeout, *downloadTimeout)

	initLogging()
	zap.S().Infow("Startup configuration", "config", *configFile, "cloud", *cloudName, "verbose", *verbose, "no_color", *noColor, "owner_project_id", *ownerProjectID, "require_protected", *requireProtected, "require_public", *requirePublic, "upload_timeout_secs", *uploadTimeout, "download_timeout_secs", *downloadTimeout)

	c := config.Load(*configFile)
	zap.S().Infow("Loaded images configuration", "path", *configFile, "image_count", len(c.Images))

	var sc *gophercloud.ServiceClient = client.New(*cloudName)
	zap.S().Infow("OpenStack client initialized", "service", "image", "cloud", *cloudName)
	if *ownerProjectID != "" {
		_ = os.Setenv("IMAGE_SHEPHERD_OWNER_PROJECT_ID", *ownerProjectID)
		zap.S().Infow("Applied owner constraint", "owner_project_id", *ownerProjectID)
	}
	if *requireProtected {
		_ = os.Setenv("IMAGE_SHEPHERD_REQUIRE_PROTECTED", "true")
		zap.S().Infow("Applied protected constraint", "require_protected", true)
	} else {
		_ = os.Setenv("IMAGE_SHEPHERD_REQUIRE_PROTECTED", "false")
	}
	if *requirePublic {
		_ = os.Setenv("IMAGE_SHEPHERD_REQUIRE_PUBLIC", "true")
		zap.S().Infow("Applied public constraint", "require_public", true)
	} else {
		_ = os.Setenv("IMAGE_SHEPHERD_REQUIRE_PUBLIC", "false")
	}

	// Propagate upload and download timeouts to environment for downstream components
	_ = os.Setenv("IMAGE_SHEPHERD_UPLOAD_TIMEOUT_SECS", strconv.Itoa(*uploadTimeout))
	_ = os.Setenv("IMAGE_SHEPHERD_DOWNLOAD_TIMEOUT_SECS", strconv.Itoa(*downloadTimeout))
	zap.S().Infow("Applied upload timeout", "upload_timeout_secs", *uploadTimeout)
	zap.S().Infow("Applied download timeout", "download_timeout_secs", *downloadTimeout)

	if !*verbose {
		zap.S().Warnw("Starting shepherd run", "image_count", len(c.Images), "hint", "use -verbose for detailed logs")
	}
	shepherd.Run(sc, c.Images)
}
