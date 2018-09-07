.DEFAULT_GOAL := build_local

GOFMT=gofmt
GC=go build
VERSION := $(shell git describe --abbrev=4 --dirty --always --tags)
Minversion := $(shell date)
BUILD_NKND_PARAM = -ldflags "-s -w -X github.com/nknorg/nkn/util/config.Version=$(VERSION)" #-race
BUILD_NKNC_PARAM = -ldflags "-s -w -X github.com/nknorg/nkn/cli/common.Version=$(VERSION)"
IDENTIFIER= $(GOOS)-$(GOARCH)

help:          ## Show available options with this Makefile
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -v grep | awk 'BEGIN { FS = ":.*?##" }; { printf "%-15s  %s\n", $$1,$$2 }'

.PHONY: build
build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GC)  $(BUILD_NKND_PARAM) -o $(FLAGS)/nknd nknd.go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GC)  $(BUILD_NKNC_PARAM) -o $(FLAGS)/nknc nknc.go

.PHONY: crossbuild
crossbuild:
	mkdir -p build/$(IDENTIFIER)
	make build FLAGS="build/$(IDENTIFIER)"

.PHONY: all
all: vendor ## Build binaries for all available architectures
	make crossbuild GOOS=linux GOARCH=arm
	make crossbuild GOOS=linux GOARCH=386
	make crossbuild GOOS=linux GOARCH=arm64
	make crossbuild GOOS=linux GOARCH=amd64
	make crossbuild GOOS=darwin GOARCH=amd64
	make crossbuild GOOS=darwin GOARCH=386
	make crossbuild GOOS=windows GOARCH=amd64
	make crossbuild GOOS=windows GOARCH=386

.PHONY: build_local
build_local: vendor ## Build local binaries without providing specific GOOS/GOARCH
	$(GC)  $(BUILD_NKND_PARAM) nknd.go
	$(GC)  $(BUILD_NKNC_PARAM) nknc.go

.PHONY: format
format:   ## Run go format on nknd.go
	$(GOFMT) -w nknd.go

.PHONY: glide
glide:   ## Installs glide for go package management
	@ mkdir -p $$(go env GOPATH)/bin
	@ curl https://glide.sh/get | sh;

vendor: glide.yaml glide.lock
	@ glide install

.PHONY: test
test:  ## Run go test
	go test -v github.com/nknorg/nkn/common
	go test -v github.com/nknorg/nkn/net
	go test -v github.com/nknorg/nkn/por
	go test -v github.com/nknorg/nkn/db
	go test -v github.com/nknorg/nkn/cli

.PHONY: clean
clean:  ## Remove the nknd, nknc binaries and build directory
	rm -rf nknd nknc
	rm -rf build/

.PHONY: deepclean
deepclean:  ## Remove the existing binaries, the vendor directory and build directory
	rm -rf nknd nknc vendor build
