GOFMT=gofmt
GC=go build
VERSION := $(shell git describe --abbrev=4 --dirty --always --tags)
Minversion := $(shell date)
BUILD_NKND_PARAM = -ldflags "-X github.com/nknorg/nkn/util/config.Version=$(VERSION)" #-race
BUILD_NKNC_PARAM = -ldflags "-X github.com/nknorg/nkn/cli/common.Version=$(VERSION)"

.PHONY: nknd
nknd: vendor
	$(GC)  $(BUILD_NKND_PARAM) nknd.go

.PHONY: all
all: vendor
	$(GC)  $(BUILD_NKND_PARAM) nknd.go
	$(GC)  $(BUILD_NKNC_PARAM) nknc.go

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
