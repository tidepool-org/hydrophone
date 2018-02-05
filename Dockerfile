# Development
FROM golang:1.9.2-alpine AS development

WORKDIR /go/src/github.com/tidepool-org/hydrophone

COPY . .

RUN  ./build.sh

CMD ["./dist/hydrophone"]

# Release
FROM alpine:latest AS release

RUN ["apk", "add", "--no-cache", "ca-certificates"]

RUN ["adduser", "-D", "tidepool"]

WORKDIR /home/tidepool

USER tidepool

COPY --from=development --chown=tidepool /go/src/github.com/tidepool-org/hydrophone/dist/hydrophone .

CMD ["./hydrophone"]
