#!/bin/sh -eu

go get -u github.com/swaggo/swag/cmd/swag
$GOPATH/bin/swag init -g ./hydrophone.go
