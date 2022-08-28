# GCP Wrapper Example

Usage example for https://github.com/sindysenorita/gcplogger

## How to Run 
prepare the GCP service account key, name it as "service_account.json" (or whatever name/filepath in config.toml)

modify config.toml, set the `project_id` with the GCP project id  

at the root package: `go run main.go -c config.toml`

try `curl --location --request GET 'localhost:8080/api/healthcheck'`
If setup goes well, you will see several logs in Cloud Logging of your project
