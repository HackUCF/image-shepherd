package image

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imagedata"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"go.uber.org/zap"
)

const uploadedFmt = "02-Jan-2006"

type Image struct {
	Name       string
	Url        string
	Public     bool
	Tags       []string
	Properties map[string]string
}

func setDefault(properties *map[string]string, key string, value string) {
	if _, exists := (*properties)[key]; !exists {
		(*properties)[key] = value
	}
}

func (i Image) Init() {
	setDefault(&i.Properties, "architecture", "x86_64")
	setDefault(&i.Properties, "hypervisor_type", "qemu")
	setDefault(&i.Properties, "vm_mode", "hvm")
	setDefault(&i.Properties, "uploaded", time.Now().Format(uploadedFmt))
	setDefault(&i.Properties, "image_family", i.Name)
}

func (i Image) Upload(c *gophercloud.ServiceClient) error {
	// Download the image
	zap.S().Infof("Downloading from %s", i.Url)
	resp, err := grab.Get(".", i.Url)
	if err != nil {
		return err
	}

	// Convert to raw
	zap.S().Info("Converting image from qcow2 to raw")
	rawFile := fmt.Sprintf("%s.raw", resp.Filename)
	cmd := exec.Command("qemu-img", "convert", "-f", "qcow2", "-O", "raw", resp.Filename, rawFile)
	err = cmd.Run()
	if err != nil {
		return err
	}

	if err = renameIfExists(c, i.Name); err != nil {
		return err
	}

	// Determine the image visibility
	var visibility images.ImageVisibility
	if i.Public {
		visibility = images.ImageVisibilityPublic
	} else {
		visibility = images.ImageVisibilityPrivate
	}

	// Create the image object
	zap.S().Info("Creating image object")
	createOpts := images.CreateOpts{
		Name:            i.Name,
		Tags:            i.Tags,
		Visibility:      &visibility,
		ContainerFormat: "bare",
		DiskFormat:      "raw",
		Properties:      i.Properties,
	}
	res, err := images.Create(c, createOpts).Extract()
	if err != nil {
		return err
	}
	zap.S().Infof("Created image %s", res.ID)

	// Upload the image data
	data, err := os.Open(rawFile)
	if err != nil {
		return err
	}
	defer data.Close()

	zap.S().Info("Uploading image data")
	return imagedata.Upload(c, res.ID, data).ExtractErr()
}

func renameIfExists(c *gophercloud.ServiceClient, name string) error {
	zap.S().Infof("Searching for existing images named %s", name)
	pages, err := images.List(c, images.ListOpts{
		Name: name,
	}).AllPages()
	if err != nil {
		return err
	}

	allImages, err := images.ExtractImages(pages)
	if err != nil {
		return err
	}

	for _, i := range allImages {
		zap.S().Infof("Updating name of image %s", i.ID)
		date, exists := i.Properties["uploaded"].(string)
		if !exists {
			zap.S().Warnf("Image has no `uploaded` tag, falling back to creation time")
			date = i.CreatedAt.Format(uploadedFmt)
		}

		newName := fmt.Sprintf("%s-%s", i.Name, date)

		updateOpts := images.UpdateOpts{
			images.ReplaceImageName{
				NewName: newName,
			},
			images.ReplaceImageHidden{
				NewHidden: true,
			},
		}
		_, err = images.Update(c, i.ID, updateOpts).Extract()
		if err != nil {
			return err
		}
		zap.S().Infof("Changed name of image %s to %s and set os_hidden=True", i.Name, newName)
	}

	return nil
}
