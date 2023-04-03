# Development
FROM golang:1.19-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/hydrophone
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/hydrophone
USER tidepool
RUN go install github.com/cosmtrek/air@latest
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

#ENV TIDEPOOL_HYDROPHONE_SERVICE='{"hydrophone":{"i18nTemplatesPath": "/home/tidepool/templates","assetUrl":"http://exadoctor.s3-website.us-east-2.amazonaws.com"}}'
USER tidepool
ENV GO111MODULE=on
COPY --from=development --chown=tidepool /go/src/github.com/tidepool-org/hydrophone/dist/hydrophone .
COPY --chown=tidepool templates/html ./templates/html/
COPY --chown=tidepool templates/locales ./templates/locales/
COPY --chown=tidepool templates/meta ./templates/meta/
COPY --chown=tidepool env.sh .
RUN ./env.sh
# ARG myassetUrl='{"hydrophone":{"myasset":"https://exadoctor.s3-website.us-east-2.amazonaws.com/"}}'
# ENV myassetUrl1=$myassetUrl
# RUN echo $TIDEPOOL_HYDROPHONE_SERVICE
# RUN echo $myassetUrl
CMD ["./hydrophone"]
