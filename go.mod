module github.com/tidepool-org/hydrophone

go 1.15

replace github.com/tidepool-org/go-common => github.com/mdblp/go-common v0.7.2-0.20210323141933-6b225f5dacf1

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/aws/aws-sdk-go v1.34.24
	github.com/gorilla/mux v1.8.0
	github.com/mdblp/crew v0.2.1-0.20210519164127-29a9da20595d
	// Retrieved through go get github.com/mdblp/shoreline/clients/shoreline@dblp.1.5.1
	github.com/mdblp/shoreline v0.14.2-0.20210503074837-5c41e0d08861
	github.com/nicksnyder/go-i18n/v2 v2.0.3
	github.com/swaggo/swag v1.7.0
	github.com/tidepool-org/go-common v0.0.0-00010101000000-000000000000
	go.mongodb.org/mongo-driver v1.4.1
	golang.org/x/text v0.3.5
	gopkg.in/yaml.v2 v2.4.0
)
