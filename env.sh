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

# Use this below to override local AWS credentials. Otherwise local credentials will be used so the user/profile needs to have rights for sending emails
# export "AWS_PROFILE" = "${NON_DEFAULT_PROFILE}" for using a .aws/credentials non default profile
# OR
# export "AWS_ACCESS_KEY_ID" = "${AWS_ACCESS_KEY_ID}"
# export "AWS_SECRET_ACCESS_KEY" = "${AWS_SECRET_ACCESS_KEY}"

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
        "supportUrl": "mailto:yourloops@diabeloop.fr",
        "assetUrl": "https://s3-eu-west-1.amazonaws.com/com.diabeloop.public-assets",
        "i18nTemplatesPath": "/go/src/github.com/tidepool-org/hydrophone/templates",
        "allowPatientResetPassword": true,
        "patientPasswordResetUrl": "https://diabeloop.zendesk.com/hc/articles/360021365373"
    },
    "sesEmail" : {
        "region":"eu-west-1",
        "fromAddress": "${SUPPORT_EMAIL_ADDR}"
        "serverEndpoint": ""
    }
}'