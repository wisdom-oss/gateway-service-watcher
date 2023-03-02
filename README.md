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
* Healthcheck &rarr; `wisdom-oss.service.healthcheck` (accepts bool)

## Usage
This tool connects to the docker daemon under `/var/run/docker.sock` and looks 
up all the containers labeled with `wisdom-oss.isService=true`. It then queries
the information endpoint of a container `/_info` and/or uses the values from the
other labels to register the service at the Kong API Gateway

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

  # A service which uses the inbuilt info path to allow the setup
  autoService:
    image: thisisamicroservice:latest
    labels:
      wisdom-oss.isService: true
  
  # A service which uses only the lables to be configured  
  manualService:
    image: thisisanothermicroserivce:latest
    labels:
      wisdom-oss.isService: true
      wisdom-oss.service.name: someName
      wisdom-oss.service.path: /somePath
      wisdom-oss.service.upstream-name: someUpstream
      wisdom-oss.service.healthcheck: false
```