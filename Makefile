GOFMT=gofmt
GC=go build
VERSION := $(shell git describe --abbrev=4 --dirty --always --tags)
Minversion := $(shell date)
BUILD_NKND_PARAM = -ldflags "-s -w -X github.com/nknorg/nkn/util/config.Version=$(VERSION)" #-race
BUILD_NKNC_PARAM = -ldflags "-s -w -X github.com/nknorg/nkn/cli/common.Version=$(VERSION)"
IDENTIFIER= $(GOOS)-$(GOARCH)

.PHONY: nknd
nknd: vendor
	$(GC)  $(BUILD_NKND_PARAM) nknd.go

.PHONY: build
build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GC)  $(BUILD_NKND_PARAM) -o $(FLAGS)/nknd-$(GOOS)-$(GOARCH) nknd.go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GC)  $(BUILD_NKNC_PARAM) -o $(FLAGS)/nknc-$(GOOS)-$(GOARCH) nknc.go

.PHONY: crossbuild
crossbuild:
	mkdir -p build/$(IDENTIFIER)
	make build FLAGS="build/$(IDENTIFIER)"

.PHONY: all
all: vendor
	make crossbuild GOOS=linux GOARCH=arm
	make crossbuild GOOS=linux GOARCH=386
	make crossbuild GOOS=linux GOARCH=arm64
	make crossbuild GOOS=linux GOARCH=amd64
	make crossbuild GOOS=darwin GOARCH=amd64
	make crossbuild GOOS=darwin GOARCH=386
	make crossbuild GOOS=windows GOARCH=amd64
	make crossbuild GOOS=windows GOARCH=386

.PHONY: format
format:
	$(GOFMT) -w nknd.go

.PHONY: glide
glide:
	@ mkdir -p $$GOPATH/bin
	@ curl https://glide.sh/get | sh;

vendor: glide.yaml glide.lock
	@ glide install

.PHONY: test
test:
	go test -v github.com/nknorg/nkn/common
	go test -v github.com/nknorg/nkn/net
	go test -v github.com/nknorg/nkn/por
	go test -v github.com/nknorg/nkn/db
	go test -v github.com/nknorg/nkn/cli

.PHONY: clean
clean:
	rm -rf nknd nknc

.PHONY: deepclean
deepclean:
	rm -rf nknd nknc vendor
