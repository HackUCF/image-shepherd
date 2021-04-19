package shepherd

import (
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/s-newman/image-shepherd/pkg/image"
)

func Run(c *gophercloud.ServiceClient, images []image.Image) {
	for _, i := range images {
		i.Init()

		err := i.Upload(c)
		if err != nil {
			fmt.Printf("Failed to upload image %s: %s\n", i.Name, err)
		} else {
			fmt.Printf("Image %s uploaded\n", i.Name)
		}
	}
}
