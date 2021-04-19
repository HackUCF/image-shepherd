package := github.com/s-newman/image-shepherd/cmd/shepherd
build_name := image-shepherd
build_dir := build

.PHONY: build dist clean

define build_dist
	GOOS=$(1) GOARCH=$(2) CGO_ENABLED=0 go build -o $(build_dir)/$(build_name)-$(1)-$(2) $(package)
	zip -j $(build_dir)/$(build_name)-$(1)-$(2).zip $(build_dir)/$(build_name)-$(1)-$(2)
endef

build:
	CGO_ENABLED=0 go build -o $(build_name) $(package)

dist:
	$(call build_dist,linux,amd64)
	$(call build_dist,windows,amd64)
	$(call build_dist,darwin,amd64)
	$(call build_dist,darwin,arm64)

clean:
	rm -f $(build_dir)/* $(build_name)