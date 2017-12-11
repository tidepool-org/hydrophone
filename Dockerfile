FROM golang:1.9.1-alpine

# Common ENV
ENV API_SECRET="This is a local API secret for everyone. BsscSHqSHiwrBMJsEGqbvXiuIUPAjQXU" \
    SERVER_SECRET="This needs to be the same secret everywhere. YaHut75NsK1f9UKUXuWqxNN0RUwHFBCy" \
    LONGTERM_KEY="abcdefghijklmnopqrstuvwxyz" \
    DISCOVERY_HOST=hakken:8000 \
    PUBLISH_HOST=hakken \
    METRICS_SERVICE="{ \"type\": \"static\", \"hosts\": [{ \"protocol\": \"http\", \"host\": \"highwater:9191\" }] }" \
    USER_API_SERVICE="{ \"type\": \"static\", \"hosts\": [{ \"protocol\": \"http\", \"host\": \"shoreline:9107\" }] }" \
    SEAGULL_SERVICE="{ \"type\": \"static\", \"hosts\": [{ \"protocol\": \"http\", \"host\": \"seagull:9120\" }] }" \
    GATEKEEPER_SERVICE="{ \"type\": \"static\", \"hosts\": [{ \"protocol\": \"http\", \"host\": \"gatekeeper:9123\" }] }" \
# Container specific ENV
    TIDEPOOL_HYDROPHONE_ENV="{\"hakken\": { \"host\": \"hakken:8000\" },\"gatekeeper\": { \"serviceSpec\": { \"type\": \"static\", \"hosts\": [\"http://gatekeeper:9123\"] } },\"seagull\": { \"serviceSpec\": { \"type\": \"static\", \"hosts\": [\"http://seagull:9120\"] } },\"highwater\": {\"serviceSpec\": { \"type\": \"static\", \"hosts\": [\"http://highwater:9191\"] },\"name\": \"highwater\",\"metricsSource\" : \"hydrophone-local\",\"metricsVersion\" : \"v0.0.1\"},\"shoreline\": {\"serviceSpec\": { \"type\": \"static\", \"hosts\": [\"http://shoreline:9107\"] },\"name\": \"hydrophone-local\",\"secret\": \"This needs to be the same secret everywhere. YaHut75NsK1f9UKUXuWqxNN0RUwHFBCy\",\"tokenRefreshInterval\": \"1h\"}}" \
    TIDEPOOL_HYDROPHONE_SERVICE="{\"service\": {\"service\": \"hydrophone-local\",\"protocol\": \"http\",\"host\": \"localhost:9157\",\"keyFile\": \"config/key.pem\",\"certFile\": \"config/cert.pem\"},\"mongo\": {\"connectionString\": \"mongodb://mongo/confirm\"},\"hydrophone\" : {\"serverSecret\": \"This needs to be the same secret everywhere. YaHut75NsK1f9UKUXuWqxNN0RUwHFBCy\",\"webUrl\": \"http://localhost:3000\",\"assetUrl\": \"https://s3-us-west-2.amazonaws.com/tidepool-dev-asset\"},\"sesEmail\" : {\"serverEndpoint\":\"https://email.us-west-2.amazonaws.com\",\"fromAddress\" : \"AWS_AUTHENTICATED_EMAIL\",\"accessKey\": \"AWS_KEY\",\"secretKey\": \"AWS_SECRET\"}}"


WORKDIR /go/src/github.com/tidepool-org/hydrophone

COPY . /go/src/github.com/tidepool-org/hydrophone

# Update config to work with Docker hostnames
RUN ./build.sh && rm -rf .git .gitignore

CMD ["./dist/hydrophone"]
