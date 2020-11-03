# Development
FROM golang:1.15-alpine AS development
ENV GO111MODULE=on

WORKDIR /go/src/github.com/tidepool-org/hydrophone

COPY . .

RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk add git rsync

RUN  ./build.sh

CMD ["./dist/hydrophone"]

# Production
FROM alpine:latest AS production
WORKDIR /home/tidepool
RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk add --no-cache ca-certificates && \
    adduser -D tidepool
USER tidepool
ENV GO111MODULE=on
COPY --from=development --chown=tidepool /go/src/github.com/tidepool-org/hydrophone/dist/hydrophone .
COPY --chown=tidepool templates/html ./templates/html/
COPY --chown=tidepool templates/locales ./templates/locales/
COPY --chown=tidepool templates/meta ./templates/meta/

CMD ["./hydrophone"]
