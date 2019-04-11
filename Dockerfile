# Development
FROM golang:1.11.4-alpine AS development

WORKDIR /go/src/github.com/tidepool-org/hydrophone

COPY . .

RUN  ./build.sh

CMD ["./dist/hydrophone"]

# Release
FROM alpine:latest AS release

RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk add --no-cache ca-certificates && \
    adduser -D tidepool

WORKDIR /home/tidepool

USER tidepool

COPY --from=development --chown=tidepool /go/src/github.com/tidepool-org/hydrophone/dist/hydrophone .

CMD ["./hydrophone"]
