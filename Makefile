MAKEFLAGS += --silent

all:

lint:
	docker run -t --rm -v $(PWD):/app -w /app golangci/golangci-lint:v1.58.0 golangci-lint run -v -D errcheck

deepcopy:
	go run tools/deepcopy-gen/deepcopy-gen.go \
		Config Status InterfaceConfig \
		InterfaceStatus PrefixConfig RouteConfig \
		RDNSSConfig DNSSLConfig

check-deepcopy:
	$(MAKE) deepcopy
	git diff --exit-code zz_generated_deepcopy.go || echo "deepcopy is not up to date. Please commit the changes."; git diff zz_generated_deepcopy.go

coverage:
	go test -cover -coverprofile=coverage.out -v
	go tool cover -html=coverage.out -o coverage.html
