package main

import (
	"os"
	"path"

	"github.com/globusdigital/deep-copy/deepcopy"
	"golang.org/x/tools/go/packages"
)

const fileName = "zz_generated_deepcopy.go"

func main() {
	// Hack: go run generates different Args[0] everytime. This makes the
	// output of this tool different everytime because of the DON'T EDIT
	// header. To make it consistent, we set Args[0] to a constant value.
	os.Args[0] = "deepcopy-gen"

	types := os.Args[1:]

	generator := deepcopy.NewGenerator(
		deepcopy.IsPtrRecv(true),
		deepcopy.WithMethodName("deepCopy"),
	)

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	f, err := os.Create(path.Join(wd, fileName))
	if err != nil {
		panic(err)
	}

	pkgs, err := packages.Load(
		&packages.Config{
			Mode: packages.NeedName |
				packages.NeedFiles |
				packages.NeedTypes |
				packages.NeedTypesInfo |
				packages.NeedDeps |
				packages.NeedImports,
		},
		"",
	)
	if err != nil {
		panic(err)
	}

	if err := generator.Generate(f, types, pkgs[0]); err != nil {
		panic(err)
	}
}
