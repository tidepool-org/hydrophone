#!/bin/sh -eu

export TEMPLATE_PATH="$(dirname $(readlink -f $0))/templates"
for D in $(find . -name '*_test.go' ! -path './vendor/*' | cut -f2 -d'/' | uniq); do
    go test -v ./...
done
