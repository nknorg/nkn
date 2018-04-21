GOFMT=gofmt
GC=go build
VERSION := $(shell git describe --abbrev=4 --dirty --always --tags)
Minversion := $(shell date)
BUILD_NODE_PAR = -ldflags "-X nkn/common/config.Version=$(VERSION)" #-race
BUILD_NODECTL_PAR = -ldflags "-X main.Version=$(VERSION)"

.PHONY: node
node:
	$(GC)  $(BUILD_NODE_PAR) -o node main.go

.PHONY: all
all:
	$(GC)  $(BUILD_NODE_PAR) -o node main.go
	$(GC)  $(BUILD_NODECTL_PAR) nodectl.go

.PHONY: format
format:
	$(GOFMT) -w main.go

.PHONY: glide
glide:
	@ mkdir -p $$GOPATH/bin
	@ curl https://glide.sh/get | sh;

.PHONY: vendor
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
	rm -rf node nodectl

.PHONY: deepclean
deepclean:
	rm -rf node nodectl vendor
