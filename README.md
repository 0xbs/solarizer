# solarizer

API Server and Influx Importer for SolarWeb.


## Build

Setup Golang, download dependencies and run a build:
```shell
brew install go
go mod download
go build
```

To build the container image, install and run ko:
```shell
brew install ko
ko build --local
```


## Run

### Docker Compose
```yaml
solarizer:
    image: ghcr.io/0xbs/solarizer:latest
    container_name: solarizer
    restart: unless-stopped
    environment:
      TZ: "Europe/Berlin"
      API_TOKENS: "TODO"
      SOLAR_WEB_PV_SYSTEM_ID: "TODO"
      SOLAR_WEB_USERNAME: "TODO"
      SOLAR_WEB_PASSWORD: "TODO"
      INFLUX_URL: "TODO"
      INFLUX_TOKEN: "TODO"
      INFLUX_ORG: "TODO"
      INFLUX_BUCKET: "TODO"
    volumes:
      - "./solarizer:/tmp/solarizer"
    ports:
      - 8080:8080
```

### Environment variables

| Name                       | Description                                                                  |
|----------------------------|------------------------------------------------------------------------------|
| API_TOKENS                 | Comma-separated list of arbitrary tokens to authenticate                     |
| DISABLE_API_SERVER         | (optional) Set to "true" to disable the API server                           |
| DISABLE_INFLUX_IMPORTER    | (optional) Set to "true" to disable the Influx importer                      |
| INFLUX_URL                 | URL of the influx database                                                   |
| INFLUX_TOKEN               | API token of the influx database                                             |
| INFLUX_ORG                 | Organization name                                                            |
| INFLUX_BUCKET              | Bucket name                                                                  |
| SOLAR_WEB_PV_SYSTEM_ID     | SolarWeb PV System ID found in the URL                                       |
| SOLAR_WEB_AUTH_COOKIE      | (optional) Value of the auth cookie for initial run                          |
| SOLAR_WEB_AUTH_COOKIE_FILE | (optional) Path and filename to the a file where the auth cookie is stored   |
| SOLAR_WEB_USERNAME         | SolarWeb/Fronius username for automatic re-login                             |
| SOLAR_WEB_PASSWORD         | SolarWeb/Fronius password for automatic re-login                             |

If `SOLAR_WEB_USERNAME` and `SOLAR_WEB_PASSWORD` are set, `solarizer` will try to perform an automatic login when SolarWeb redirects requests back to the login flow because the auth cookie expired. The refreshed `.AspNet.Auth` cookie is then persisted in `SOLAR_WEB_AUTH_COOKIE_FILE` as before.

By default, both the API server and the Influx importer are enabled. Set `DISABLE_API_SERVER=true` or `DISABLE_INFLUX_IMPORTER=true` to turn them off.


## API

### Endpoints

| Endpoint                 | Description                                         |
|--------------------------|-----------------------------------------------------|
| `PUT /api/auth/cookie`.  | Set new auth cookie value given in the request body |
| `GET /api/pv/power`      | Get power data                                      |
| `GET /api/pv/production` | Get earnings and productions data                   |
| `GET /api/pv/balance`    | Get grid balance data                               |

### Example

```shell
curl --request PUT \
  --location 'https://HOSTNAME/api/auth/cookie' \
  --header 'Authorization: Bearer APITOKEN' \
  --data 'COOKIEVALUE'

curl --location 'https://HOSTNAME/api/pv/power' --header 'Authorization: Bearer APITOKEN'
```
