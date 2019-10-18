module github.com/tidepool-org/hydrophone

go 1.11.2

replace github.com/tidepool-org/go-common => github.com/mdblp/go-common v0.1.1-config-from-env.1.0.20191018064735-58c518ef96ca

require (
	github.com/aws/aws-sdk-go v1.25.5
	github.com/globalsign/mgo v0.0.0-20181015135952-eeefdecb41b8
	github.com/gorilla/mux v1.7.3
	github.com/nicksnyder/go-i18n/v2 v2.0.2
	github.com/stretchr/testify v1.4.0 // indirect
	github.com/tidepool-org/go-common v0.0.0
	golang.org/x/net v0.0.0-20191002035440-2ec189313ef0 // indirect
	golang.org/x/text v0.3.2
	gopkg.in/yaml.v2 v2.2.2
)
