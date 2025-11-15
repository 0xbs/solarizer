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

| Name                       | Description                                                                |
|----------------------------|----------------------------------------------------------------------------|
| API_TOKENS                 | Comma-separated list of arbitrary tokens to authenticate                   |
| INFLUX_URL                 | URL of the influx database                                                 |
| INFLUX_TOKEN               | API token of the influx database                                           |
| INFLUX_ORG                 | Organization name                                                          |
| INFLUX_BUCKET              | Bucket name                                                                |
| SOLAR_WEB_PV_SYSTEM_ID     | SolarWeb PV System ID found in the URL                                     |
| SOLAR_WEB_AUTH_COOKIE      | (optional) Value of the auth cookie for initial run                        |
| SOLAR_WEB_AUTH_COOKIE_FILE | (optional) Path and filename to the a file where the auth cookie is stored |


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
