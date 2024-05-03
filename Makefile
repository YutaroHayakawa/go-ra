MAKEFLAGS += --silent

DEEPCOPY_OPTS=-type Config -type InterfaceConfig -type InterfaceStatus

all:

deepcopy:
	deep-copy -pointer-receiver $(DEEPCOPY_OPTS) -o zz_generated_deepcopy.go .

check-deepcopy:
	deep-copy -pointer-receiver $(DEEPCOPY_OPTS) -o zz_generated_deepcopy.go .
	git diff --exit-code zz_generated_deepcopy.go || echo "deepcopy is not up to date. Please commit the changes."; git diff zz_generated_deepcopy.go
