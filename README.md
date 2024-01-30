<div align="center">
<img height="150px" src="https://raw.githubusercontent.com/wisdom-oss/brand/main/svg/standalone_color.svg">
<h1>Gateway Service Watchdog</h1>
<h3>service-watchdog</h3>
<p>👀 microservice provisioning and monitoring</p>
<img alt="GitHub Actions Workflow Status" src="https://img.shields.io/github/actions/workflow/status/wisdom-oss/gateway-service-watcher/docker.yaml?style=for-the-badge&label=Docker%20Build">
<img src="https://img.shields.io/github/go-mod/go-version/wisdom-oss/gateway-service-watcher?style=for-the-badge" alt="Go Lang Version"/>
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

> [!CAUTION]
> Changing the OpenID Connect configuration in the watchdog only will result
> in a locked platform since the frontend is unable to receive the required
> values dynamically. Remember to rebuild the frontend after changing the values.
