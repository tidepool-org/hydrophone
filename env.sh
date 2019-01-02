export TIDEPOOL_HYDROPHONE_ENV='{
    "hakken": { "host": "localhost:8000" },
    "gatekeeper": { "serviceSpec": { "type": "static", "hosts": ["http://localhost:9123"] } },
    "seagull": { "serviceSpec": { "type": "static", "hosts": ["http://localhost:9120"] } },
    "highwater": {
  	    "serviceSpec": { "type": "static", "hosts": ["http://localhost:9191"] },
  	    "name": "highwater",
        "metricsSource" : "hydrophone-local",
        "metricsVersion" : "v0.0.1"
    },
    "shoreline": {
        "serviceSpec": { "type": "static", "hosts": ["http://localhost:9107"] },
        "name": "hydrophone-local",
        "secret": "This needs to be the same secret everywhere. YaHut75NsK1f9UKUXuWqxNN0RUwHFBCy",
        "tokenRefreshInterval": "1h"
    }
}'

export TIDEPOOL_HYDROPHONE_SERVICE='{
    "service": {
        "service": "hydrophone-local",
        "protocol": "http",
        "host": "localhost:9157",
        "keyFile": "config/key.pem",
        "certFile": "config/cert.pem"
    },
    "mongo": {
        "connectionString": "mongodb://localhost/confirm"
    },
    "hydrophone" : {
        "serverSecret": "This needs to be the same secret everywhere. YaHut75NsK1f9UKUXuWqxNN0RUwHFBCy",
        "webUrl": "http://localhost:3000",
        "assetUrl": "https://s3-us-west-2.amazonaws.com/tidepool-dev-asset",
        "i18nTemplatesPath": "/go/src/github.com/tidepool-org/hydrophone/templates"
    },
    "sesEmail" : {
        "serverEndpoint":"https://email.us-west-2.amazonaws.com",
        "fromAddress" : "AWS_AUTHENTICATED_EMAIL",
        "accessKey": "AWS_KEY",
        "secretKey": "AWS_SECRET"
    }
}'
