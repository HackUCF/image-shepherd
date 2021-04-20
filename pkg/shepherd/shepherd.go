package shepherd

import (
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/s-newman/image-shepherd/pkg/image"
	"go.uber.org/zap"
)

func Run(c *gophercloud.ServiceClient, images []image.Image) {
	for _, i := range images {
		zap.S().Infof("Managing image %s", i.Name)

		i.Init()

		err := i.Upload(c)
		if err != nil {
			fmt.Printf("Failed to upload image %s: %s\n", i.Name, err)
		} else {
			fmt.Printf("Image %s uploaded\n", i.Name)
		}
	}
}
