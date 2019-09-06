# Development
FROM golang:1.12.7-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/hydrophone
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/hydrophone
USER tidepool
COPY --chown=tidepool . .
RUN ./build.sh
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
CMD ["./hydrophone"]
