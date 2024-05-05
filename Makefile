MAKEFLAGS += --silent

DEEPCOPY_OPTS=-type Config -type Status -type InterfaceConfig -type InterfaceStatus

all:

lint:
	docker run -t --rm -v $(PWD):/app -w /app golangci/golangci-lint:v1.58.0 golangci-lint run -v -D errcheck

deepcopy:
	deep-copy -pointer-receiver $(DEEPCOPY_OPTS) -o zz_generated_deepcopy.go .

check-deepcopy:
	deep-copy -pointer-receiver $(DEEPCOPY_OPTS) -o zz_generated_deepcopy.go .
	git diff --exit-code zz_generated_deepcopy.go || echo "deepcopy is not up to date. Please commit the changes."; git diff zz_generated_deepcopy.go

coverage:
	go test -cover -coverprofile=coverage.out -v
	go tool cover -html=coverage.out -o coverage.html
