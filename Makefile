SHELL = /bin/sh

TOOLS_BIN = tools/bin
NPM_BIN = node_modules/.bin

OAPI_CODEGEN = $(TOOLS_BIN)/oapi-codegen
SWAGGER_CLI = $(NPM_BIN)/swagger-cli

NPM_PKG_SPECS = \
	@apidevtools/swagger-cli@^4.0.4


.PHONY: all
all: dist/hydrophone

GENERATED_SRCS = client/types.go client/client.go spec/confirm.v1.yaml

dist/hydrophone: $(GENERATED_SRCS)
	GOWORK=off ./build.sh

.PHONY: build
build:
	$(MAKE) dist/hydrophone

.PHONY: test
test:
	GOWORK=off ./test.sh

.PHONY: generate
# Generates client api
generate: $(SWAGGER_CLI) $(OAPI_CODEGEN)
	$(SWAGGER_CLI) bundle ../TidepoolApi/reference/confirm.v1.yaml -o ./spec/confirm.v1.yaml -t yaml
	$(OAPI_CODEGEN) -package=api -generate=types spec/confirm.v1.yaml > client/types.go
	$(OAPI_CODEGEN) -package=api -generate=client spec/confirm.v1.yaml > client/client.go
	cd client && go generate ./...

$(OAPI_CODEGEN):
	GOBIN=$(shell pwd)/$(TOOLS_BIN) go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.13.4

$(SWAGGER_CLI): npm-tools

.PHONY: npm-tools
npm-tools:
# When using --no-save, any dependencies not included will be deleted, so one
# has to install all the packages all at the same time. But it saves us from
# having to muck with packages.json.
	npm install --no-save --local $(NPM_PKG_SPECS)
