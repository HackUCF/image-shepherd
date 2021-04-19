package image

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/cavaliercoder/grab"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imagedata"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
)

type Image struct {
	Name       string
	Url        string
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
}

func (i Image) Upload(c *gophercloud.ServiceClient) error {
	// Download the image
	resp, err := grab.Get(".", i.Url)
	if err != nil {
		return err
	}

	// Convert to raw
	rawFile := fmt.Sprintf("%s.raw", resp.Filename)
	cmd := exec.Command("qemu-img", "convert", "-f", "qcow2", "-O", "raw", resp.Filename, rawFile)
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Create the image object
	createOpts := images.CreateOpts{
		Name:            i.Name,
		Tags:            i.Tags,
		ContainerFormat: "bare",
		DiskFormat:      "raw",
		Properties:      i.Properties,
	}
	res, err := images.Create(c, createOpts).Extract()
	if err != nil {
		return err
	}

	// Upload the image data
	data, err := os.Open(rawFile)
	if err != nil {
		return err
	}
	defer data.Close()

	return imagedata.Upload(c, res.ID, data).ExtractErr()
}
