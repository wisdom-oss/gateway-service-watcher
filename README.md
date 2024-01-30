<div align="center">
<img height="150px" src="https://raw.githubusercontent.com/wisdom-oss/brand/main/svg/standalone_color.svg">
<h1>Gateway Watchdog</h1>
<h3>watchdog</h3>
<p>👀 microservice provisioning and monitoring</p>
<img alt="GitHub Actions Workflow Status" src="https://img.shields.io/github/actions/workflow/status/wisdom-oss/watchdog/docker.yaml?style=for-the-badge&label=Docker%20Build">
<img src="https://img.shields.io/github/go-mod/go-version/wisdom-oss/watchdog?style=for-the-badge" alt="Go Lang Version"/>
<img alt="GitHub Actions Workflow Status" src="https://img.shields.io/github/actions/workflow/status/wisdom-oss/watchdog/code-ql.yaml?style=for-the-badge&label=Code QL">
</div>

> [!NOTE]
> This service is automatically enabled and needs no further configuration to
> run in a normal deployment of the WISdoM platform.

This service utilizes the docker host it runs on to check for the containers
used in the WISdoM Platform and manages its registration in the api gateway
removing the need of manually registering the service or coding the registration
logic in each microservice.

Furthermore, the watchdog configures the authentication for accessing the
backend using the same information as used in the frontend.

> [!IMPORTANT]
> Changing the OpenID Connect configuration in the watchdog only will result
> in a locked platform since the frontend is unable to receive the required
> values dynamically. 
> Remember to rebuild the frontend after changing the values.

## Configuring
Since the watchdog interacts with the API gateway to configure the
authentication on the microservices it needs to know the client id and the
discovery endpoint of the OpenID Connect server.
These values need to be provided by the following environment variables:
  - `OIDC_CLIENT_ID` for the client id
  - `OIDC_DISCOVERY_URL` for the discovery url

These values are configured _as-is_ for the plugin instances.
The only check that is done is that the values need to be present in the
environment.

### Setting up containers
To make a container discoverable you need to set some labels to the container in
your docker compose files.
These labels help identifying containers as microservices and how to reach these
services.

> [!NOTE]
> Containers, that are not in the `wisdom` network created by the WISdoM
> platform on its deployment are automatically attached to the network if they
> are tagged as service (see below).

#### Required labels
`wisdom.service=<bool>` marks the container (or its replicas) as a 
microservice which shall be managed by the watchdog.
Setting this to `false` indicates that this service shall not be managed by the
watchdog.<br>

`wisdom.service.gateway-path=<string>` indicates under which path the service
shall be available.
The path may not contain a slash as it will be automatically prepended by the
watchdog

#### Optional labels
`wisdom.service.port=<uint16>` indicates to the watchdog, which port it needs to
use while registering the microservice in the gateway to ensure its reachability.
If this value is not set, the watchdog assumes the port to be `8000` since this
is the default value for the microservices written in 
[the organization](https://github.com/wisdom-oss).<br>
Default value: `8000`

`wisdom.service.use-auth=<bool>` indicates to the watchdog, if the
microservice is to be protected with the 
[OIDC plugin](https://github.com/wisdom-oss/api-gateway/tree/main/plugins/oidc)
requiring users to authenticate before accessing this microservice using the
API gateway.<br>
Default value: `true`
> [!CAUTION]
> Disabling the authentication in a microservice will open up your deployed
> instance to data leaks and other unauthenticated accesses.
