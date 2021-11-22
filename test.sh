#!/bin/sh

export TEMPLATE_PATH="$(dirname $(readlink -f $0))/templates"

go get -u github.com/jstemmer/go-junit-report
go get github.com/t-yuki/gocover-cobertura
go test -v -race -coverprofile=coverage.out ./... 2>&1 > test-report.txt
testPass=$?
cat test-report.txt
cat test-report.txt | go-junit-report  > test-report.xml

if [ $testPass -eq 1 ]; then
  echo "Test failled"
fi

gocover-cobertura < coverage.out > coverage.xml
go tool cover -html='coverage.out' -o coverage.html

exit $testPass
