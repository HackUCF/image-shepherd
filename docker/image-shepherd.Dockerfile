# -----------------------------------------------------------------------------
FROM golang:1.16 AS build
WORKDIR /app

# Install dependencies
COPY go.mod go.sum Makefile ./
RUN make deps

# Build app
COPY . .
RUN make build

# -----------------------------------------------------------------------------
FROM ubuntu:latest as dist
WORKDIR /opt/image-shepherd

# Install qemu-image tool
RUN apt-get update && apt-get install -y qemu-utils && rm -rf /var/lib/apt/lists

COPY --from=build /app/image-shepherd .
ENTRYPOINT ["/opt/image-shepherd/image-shepherd"]
CMD ["-no-color", "-verbose"]