# solarizer

API Server and Influx Importer for SolarWeb.

## Build

Setup Golang and Ko for easy Go containers:
```shell
brew install go ko
```

Download dependencies and build:
```shell
go mod download
go build
```

## Run

Set up environment variables:
```yaml
API_TOKENS: "f6ac06b86a5949abaa2efe333b0d2cef,347ff1b5e5f04097b8a947ebc72608a2"
SOLAR_WEB_PV_SYSTEM_ID: "331ec2e4-2065-4188-97f8-96dd43d61870"
SOLAR_WEB_AUTH_COOKIE: "optional-cookie-value"
INFLUX_URL: "https://influx.example.com"
INFLUX_TOKEN: "my-influx-token"
INFLUX_ORG: "my-influx-org"
INFLUX_BUCKET: "my-influx-bucket"
```
