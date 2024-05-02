MAKEFLAGS += --silent

all:

deepcopy:
	deep-copy -pointer-receiver --type Config --type InterfaceConfig -o zz_generated_deepcopy.go .

check-deepcopy:
	deep-copy -pointer-receiver --type Config --type InterfaceConfig -o zz_generated_deepcopy.go .
	git diff --exit-code zz_generated_deepcopy.go || echo "deepcopy is not up to date. Please commit the changes."; git diff zz_generated_deepcopy.go
