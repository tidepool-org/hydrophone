#!/bin/sh -eu

rm -rf dist
mkdir dist

# generate version number
if [ -n "${TRAVIS_TAG:-}" ]; then
    VERSION_BASE=${TRAVIS_TAG}  
else 
    VERSION_BASE=$(git describe --abbrev=0 --tags 2> /dev/null || echo 'dblp.0.0.0')
fi
VERSION_SHORT_COMMIT=$(git rev-parse --short HEAD)
VERSION_FULL_COMMIT=$(git rev-parse HEAD)

GO_COMMON_PATH="github.com/tidepool-org/go-common"
	
echo "Build hydrophone preview $VERSION_BASE+$VERSION_FULL_COMMIT"
go mod tidy
go build -o dist/hydro-preview

cp env.sh dist/
cp start.sh dist/

cp index.html dist/index.html
echo "Push email templates"
rsync -av --progress ../ dist/ --exclude '*.go' --exclude 'preview'


