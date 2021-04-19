Image Puller
============

Pull prebuilt cloud images.

Convert qcow2 to raw:

```shell
qemu-img convert -f qcow2 -O raw <qcow2-file> <raw-file>
```

Upload image:

```shell
openstack --os-cloud openstack image create --file <raw-file> \
  --property architecture=x86_64 \
  --property hypervisor_type=qemu \
  --property os_distro=<distro-name>  \
  --property os_version=<version-number> \
  --property vm_mode=hvm \
  <image-name>
```

See the `os_distro` row of [this table](https://docs.openstack.org/glance/rocky/admin/useful-image-properties.html#image-property-keys-and-values) for guidance on the distro name.