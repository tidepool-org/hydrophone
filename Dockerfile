# Development
FROM golang:1.9.2-alpine AS development

WORKDIR /go/src/github.com/tidepool-org/hydrophone

COPY . .

RUN  ./build.sh

CMD ["./dist/hydrophone"]

# Release
FROM alpine:latest AS release

RUN ["apk", "add", "--no-cache", "ca-certificates"]

RUN ["adduser", "-D", "hydrophone"]

WORKDIR /home/hydrophone

USER hydrophone

COPY --from=development --chown=hydrophone /go/src/github.com/tidepool-org/hydrophone/dist/hydrophone .

CMD ["./hydrophone"]
