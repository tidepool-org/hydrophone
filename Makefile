# Clinic Makefile

# Generates client api
generate:
	swagger-cli bundle ../TidepoolApi/reference/confirm.v1.yaml -o ./spec/confirm.v1.yaml -t yaml
	oapi-codegen -package=api -generate=types spec/confirm.v1.yaml > client/types.go
	oapi-codegen -package=api -generate=client spec/confirm.v1.yaml > client/client.go
	cd client && go generate ./...

