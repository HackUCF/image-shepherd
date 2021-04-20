# Image Shepherd

[![Build container](https://github.com/s-newman/image-shepherd/actions/workflows/containers.yml/badge.svg)](https://github.com/s-newman/image-shepherd/actions/workflows/containers.yml)

Image Shepherd is a utility for adding prebuilt cloud images to OpenStack.

Image Shepherd will download prebuilt qcow2 images from sources you define, convert them to raw images, and then upload them to your OpenStack cloud. It is intended to be used on an automated schedule, such as in a cron job or in a scheduled CI pipeline. That way, you can always have a set of updated images ready to go in your OpenStack cloud.

## Installation

Image Shepherd is available as a docker container.

```shell
docker pull ghcr.io/s-newman/image-shepherd/image-shepherd:latest
```

Precompiled binaries for Linux, Windows, and Mac are available to download from GitHub.

```shell
curl -O image-shepherd.zip https://github.com/s-newman/image-shepherd/image-shepherd/releases/latest/download/image-shepherd-linux-amd64.zip
unzip image-shepherd.zip
./image-shepherd -h
```

You can also build the binary from source using `go`.

```shell
go get https://github.com/s-newman/image-shepherd/cmd/image-shepherd
./image-shepherd -h
```

## Quickstart

The easiest way to get up and running with Image Shepherd is with a Docker container.

```shell
docker run -v $PWD/clouds.yaml:/opt/image-shepherd/clouds.yaml -v $PWD/images.yaml:/opt/image-shepherd/images.yaml ghcr.io/s-newman/image-shepherd/image-shepherd:latest
```

## Usage

To use Image Shepherd, you need a few things:

- Access to an OpenStack cloud running the Glance (image) service
- A `clouds.yaml` file containing a configuration that allows reading and modifying images
- An [`images.yaml`](#configuration) file describing the cloud images that Image Shepherd will manage for you
- [Environment variables recognized by the OpenStack SDK](https://docs.openstack.org/python-openstackclient/latest/cli/man/openstack.html#environment-variables) (optional)

### `clouds.yaml` Location

Image Shepherd will look in the following locations for the `clouds.yaml` file:

1. The path pointed to by the `OS_CLIENT_CONFIG_FILE` environment variable
2. The current directory
3. The user's configuration directory (e.g. `~/.config/openstack/clouds.yaml`)
4. The system global configuration directory (e.g. `/etc/openstack/clouds.yaml`)

Even if you are using environment variables to configure access to your OpenStack cloud, you **must** have a `clouds.yaml` file in one of the above locations.

### Picking a Cloud

By default, Image Shepherd will attempt to use the cloud configuration named `openstack` in your `clouds.yaml`. If you would like to use a different cloud configuration, pass the name of the configuration to the `-os-cloud` command-line argument.

`clouds.yaml`:
```yaml
clouds:
  my_cloud:
    auth:
      auth_url: https://stack.example.com:5000
      username: "snewman"
      password: "REDACTED"
      project_id: REDACTED
      project_name: "snewman"
      user_domain_name: "Default"
    region_name: "PrimaryRegion"
    interface: "public"
    identity_api_version: 3
```

```shell
image-shepherd -os-cloud my_cloud
```

## Configuration

The `images.yaml` configuration file tells Image Shepherd where to download images from and what to do with them.

```yaml
images:
    # The name that will be given to the image in OpenStack
  - name: ubuntu-focal

    # Where to download the qcow2 image from
    url: https://cloud-images.ubuntu.com/releases/focal/release/ubuntu-20.04-server-cloudimg-amd64.img
  
    # A list of tags to add to the image (optional)
    tags:
      - official

    # A list of properties to add to the image (optional, recommended)
    properties:
      os_distro: ubuntu
      os_version: "20.04" # Make sure to quote version numbers so YAML parses them as strings!
```

If they aren't configured, Image Shepherd will set the following properties.

| Name | Value |
| --- | --- |
| `architecture` | `x86_64` |
| `hypervisor_type` | `qemu` |
| `vm_mode` | `hvm` |
| `uploaded` | Current date in 02-Jan-2006 format |
| `image_family` | Name of image |

You can override these defaults by setting the values of the properties in your `images.yaml` file.

```yaml
images:
  - name: ubuntu-focal
    url: https://cloud-images.ubuntu.com/releases/focal/release/ubuntu-20.04-server-cloudimg-armhf.img
    properties:
      architecture: armhf
      hypervisor_type: xen # Just an example. This is actually a qemu/KVM image.
```

OpenStack uses some properties to determine how to handle an image. The Glance documentation has [a list of known properties and their supported values](https://docs.openstack.org/glance/latest/admin/useful-image-properties.html#image-property-keys-and-values) that you can set if you choose.

If your Glance service has been configured to support it, you can add custom properties to your images. This should be possible in the majority of cases; Glance allows custom properties by default.

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## License
[MIT](https://choosealicense.com/licenses/mit/)