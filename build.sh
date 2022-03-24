#!/bin/sh -e

rm -rf dist
mkdir dist

TARGETPLATFORM=$1
if ["$TARGETPLATFORM"="linux/arm64"]; then
    export GOOS=darwin
    export GOARCH=arm64
    export CGO_ENABLED=0
else
    export CGO_ENABLED=1
fi

# generate version number
if [ -n "${APP_VERSION:-}" ]; then
    VERSION_BASE=${APP_VERSION}
else 
    VERSION_BASE=$(git describe --abbrev=0 --tags 2> /dev/null || echo 'dblp.0.0.0')
fi
VERSION_SHORT_COMMIT=$(git rev-parse --short HEAD)
VERSION_FULL_COMMIT=$(git rev-parse HEAD)

GO_COMMON_PATH="github.com/mdblp/go-common"
export GOPRIVATE=github.com/mdblp/crew

echo "Build hydrophone $VERSION_BASE+$VERSION_FULL_COMMIT"
go mod tidy
go build -ldflags "-X $GO_COMMON_PATH/clients/version.ReleaseNumber=$VERSION_BASE \
    -X $GO_COMMON_PATH/clients/version.FullCommit=$VERSION_FULL_COMMIT \
    -X $GO_COMMON_PATH/clients/version.ShortCommit=$VERSION_SHORT_COMMIT" \
    -o dist/hydrophone hydrophone.go

echo "Push email templates"
rsync -av --progress templates dist/ --exclude '*.go' --exclude 'preview'
