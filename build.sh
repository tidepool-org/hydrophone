#!/bin/sh -eu

rm -rf dist
mkdir dist
go build -o dist/hydrophone hydrophone.go
