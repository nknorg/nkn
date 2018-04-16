GOFMT=gofmt
GC=go build
VERSION := $(shell git describe --abbrev=4 --dirty --always --tags)
Minversion := $(shell date)
BUILD_NODE_PAR = -ldflags "-X nkn/common/config.Version=$(VERSION)" #-race
BUILD_NODECTL_PAR = -ldflags "-X main.Version=$(VERSION)"

.PHONY: all
all:
	$(GC)  $(BUILD_NODE_PAR) -o node main.go
# $(GC)  $(BUILD_NODECTL_PAR) nodectl.go

.PHONY: format
format:
	$(GOFMT) -w main.go

.PHONY: clean
clean:
	rm -rf node nodectl
