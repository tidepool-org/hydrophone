FROM golang:1.7.1-alpine

# Common ENV
ENV API_SECRET="This is a local API secret for everyone. BsscSHqSHiwrBMJsEGqbvXiuIUPAjQXU" \
    SERVER_SECRET="This needs to be the same secret everywhere. YaHut75NsK1f9UKUXuWqxNN0RUwHFBCy" \
    LONGTERM_KEY="abcdefghijklmnopqrstuvwxyz" \
    DISCOVERY_HOST=hakken:8000 \
    PUBLISH_HOST=hakken \
    METRICS_SERVICE="{ \"type\": \"static\", \"hosts\": [{ \"protocol\": \"http\", \"host\": \"highwater:9191\" }] }" \
    USER_API_SERVICE="{ \"type\": \"static\", \"hosts\": [{ \"protocol\": \"http\", \"host\": \"shoreline:9107\" }] }" \
    SEAGULL_SERVICE="{ \"type\": \"static\", \"hosts\": [{ \"protocol\": \"http\", \"host\": \"seagull:9120\" }] }" \
    GATEKEEPER_SERVICE="{ \"type\": \"static\", \"hosts\": [{ \"protocol\": \"http\", \"host\": \"gatekeeper:9123\" }] }"

WORKDIR /go/src/github.com/tidepool-org/hydrophone

COPY . /go/src/github.com/tidepool-org/hydrophone

# Update config to work with Docker hostnames
RUN sed -i -e 's/mongodb:\/\/localhost\/confirm/mongodb:\/\/mongo\/confirm/g' config/server.json \
 && sed -i -e 's/localhost:8000/hakken:8000/g' \
           -e 's/localhost:9191/highwater:9191/g' \
           -e 's/localhost:9123/gatekeeper:9123/g' \
           -e 's/localhost:9120/seagull:9120/g' \
           -e 's/localhost:9107/shoreline:9107/g' config/env.json \
# Build
 && ./build.sh \
# Remove files no longer needed after the build to reduce fs layer size
 && rm -rf .git .gitignore

CMD ["./dist/hydrophone"]
