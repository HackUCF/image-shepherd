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

COPY --from=build /app/image-shepherd .
ENTRYPOINT ["/opt/image-shepherd/image-shepherd"]
CMD ["-no-color", "-verbose"]