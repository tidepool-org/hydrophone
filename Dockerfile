# Development
FROM golang:1.10.2-alpine AS development

WORKDIR /go/src/github.com/tidepool-org/hydrophone

COPY . .

RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk add build-base git cyrus-sasl-dev

RUN  dos2unix build.sh && ./build.sh

CMD ["./dist/hydrophone"]

# Release
FROM alpine:latest AS release

RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk add --no-cache ca-certificates && \
	apk add --no-cache libsasl	&& \
    adduser -D tidepool

WORKDIR /home/tidepool

USER tidepool

COPY --from=development --chown=tidepool /go/src/github.com/tidepool-org/hydrophone/dist/hydrophone .

CMD ["./hydrophone"]
