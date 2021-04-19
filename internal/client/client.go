package client

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"go.uber.org/zap"
)

func New(cloudName string) *gophercloud.ServiceClient {
	// Create the Glance client
	opts := &clientconfig.ClientOpts{
		Cloud: cloudName,
	}
	c, err := clientconfig.NewServiceClient("image", opts)
	if err != nil {
		zap.S().Fatalf("Failed to create glance client: %s", err)
	}
	return c
}
