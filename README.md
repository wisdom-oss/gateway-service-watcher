# WISdoM OSS - Gateway Service Watcher
![GitHub go.mod Go version (branch)](https://img.shields.io/github/go-mod/go-version/wisdom-oss/gateway-service-watcher/main?label=Version&logo=Go&style=for-the-badge)

<hr>

> This tool is still a WIP and will be updated regularly. The first working 
> release will be found on the main branch

This tool uses the Docker API to determine the containers containing
microservices using the following label: `wisdom-oss.isService=true`.
Services that are found by this method are queried for the following attributes:
* Service Name
* Access Path
* Upstream Name
* Healthcheck

which may be overwritten by using the following labels on the container:
* Service Name &rarr; `wisdom-oss.service.name` (accepts string)
* Access Path &rarr; `wisdom-oss.service.path` (accepts string)
* Upstream Name &rarr; `wisdom-oss.service.upstream-name` (accepts string)

## Usage
This tool connects to the docker daemon under `/var/run/docker.sock` and looks 
up all the containers labeled with `wisdom-oss.isService=true`. The API Gateway
needs to be labelled with `wisdom-oss.isGateway=true` to be found by this
service. Important: The service watcher only supports one API Gateway at the
same time. Therefore, only one container should be labeled as the gateway.

### Example Docker Compose file
```yaml
version: '3.9'

services:
  # Kong API Gateway
  api-gateway:
    image: kong:latest
    # Configure environment according to your setup...
    
  # The service watcher
  service-watcher:
    build: https://github.com/wisdom-oss/gateway-service-watcher.git
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
  
  # A service which uses the labels to be configured  
  manualService:
    image: thisisanothermicroserivce:latest
    labels:
      wisdom-oss.isService: true
      wisdom-oss.service.name: someName
      wisdom-oss.service.path: /somePath
      wisdom-oss.service.upstream-name: someUpstream
```