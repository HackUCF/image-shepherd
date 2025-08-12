package client

import (
	"context"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
	"go.uber.org/zap"
)

// New creates and returns an OpenStack Image (Glance) service client using
// gophercloud/v2 and utils/v2. It uses a context with timeout to avoid hanging.
// The cloudName should match an entry in your clouds.yaml.
func New(cloudName string) *gophercloud.ServiceClient {
	zap.S().Infow("Initializing OpenStack image client", "cloud", cloudName)

	opts := &clientconfig.ClientOpts{
		Cloud: cloudName,
	}

	// Use a bounded context to avoid indefinite hangs during auth or discovery
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := clientconfig.NewServiceClient(ctx, "image", opts)
	if err != nil {
		zap.S().Fatalw("Failed to create glance client", "cloud", cloudName, "error", err)
	}

	// Apply an HTTP client timeout for subsequent API calls
	c.HTTPClient.Timeout = 60 * time.Second
	zap.S().Infow("OpenStack image client initialized", "cloud", cloudName, "http_timeout_seconds", 60)

	return c
}
