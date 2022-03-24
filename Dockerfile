# Development
FROM --platform=$BUILDPLATFORM golang:1.17-alpine AS development
ARG APP_VERSION
ENV APP_VERSION=${APP_VERSION}
ENV GO111MODULE=on

WORKDIR /go/src/github.com/tidepool-org/hydrophone
ARG GITHUB_TOKEN

COPY . .

RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk add git rsync
    
RUN git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"

ARG TARGETPLATFORM
ARG BUILDPLATFORM
RUN  ./build.sh $TARGETPLATFORM

CMD ["./dist/hydrophone"]

# Production
FROM --platform=$BUILDPLATFORM alpine:latest AS production
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
