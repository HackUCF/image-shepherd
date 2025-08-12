package := github.com/HackUCF/image-shepherd/cmd/image-shepherd
build_name := image-shepherd
build_dir := build
docker_registry := ghcr.io/s-newman

.PHONY: build dist deps clean docker

build:
	CGO_ENABLED=0 go build -o $(build_name) $(package)

dist:
	scripts/go-build.sh linux amd64 $(build_dir) $(build_name) $(package)
	scripts/go-build.sh windows amd64 $(build_dir) $(build_name) $(package)
	scripts/go-build.sh darwin amd64 $(build_dir) $(build_name) $(package)
	scripts/go-build.sh darwin arm64 $(build_dir) $(build_name) $(package)

deps:
	go mod download

docker-build:
	docker build -f docker/$(build_name).Dockerfile -t $(docker_registry)/$(build_name):latest .

docker-push:
	scripts/docker-push.sh $(docker_registry)/$(build_name)

clean:
	find $(build_dir) -type f | grep -v .gitkeep | xargs rm
	rm -f $(build_name)
