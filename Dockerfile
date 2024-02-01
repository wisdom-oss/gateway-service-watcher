FROM golang:alpine AS build-service
COPY . /tmp/src
WORKDIR /tmp/src
RUN mkdir -p /tmp/build && go mod download -x && go build -v -x -o /tmp/build/app

FROM alpine:latest
COPY --from=build-service /tmp/build/app /watchdog
COPY resources/* /
ENTRYPOINT ["/watchdog"]
LABEL org.opencontainers.image.source=https://github.com/wisdom-oss/gateway-service-watcher
