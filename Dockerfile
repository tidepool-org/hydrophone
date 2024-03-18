# Development
FROM golang:1.21-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/hydrophone
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/hydrophone
USER tidepool
RUN go install github.com/cosmtrek/air@v1.49.0
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["air"]

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
