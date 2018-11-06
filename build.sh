#!/bin/sh -eu

rm -rf dist
mkdir dist
go get gopkg.in/mgo.v2
go build -o dist/hydrophone hydrophone.go
cp env.sh dist/
cp start.sh dist/
