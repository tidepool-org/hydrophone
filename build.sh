#!/bin/sh -eu

rm -rf dist
mkdir dist
go get gopkg.in/mgo.v2 github.com/nicksnyder/go-i18n/v2/i18n gopkg.in/yaml.v2
go build -o dist/hydrophone hydrophone.go
cp env.sh dist/
cp start.sh dist/
rsync -av --progress templates dist/ --exclude '*.go'