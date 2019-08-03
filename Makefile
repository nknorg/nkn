.DEFAULT_GOAL:=build_local_or_with_proxy

GC=GO111MODULE=on go build
USE_PROXY=GOPROXY=https://goproxy.io
GOFMT=go fmt
VERSION:=$(shell git describe --abbrev=4 --dirty --always --tags)
Minversion:=$(shell date)
BUILD_NKND_PARAM=-ldflags "-s -w -X github.com/nknorg/nkn/util/config.Version=$(VERSION)"
BUILD_NKNC_PARAM=-ldflags "-s -w -X github.com/nknorg/nkn/cli/common.Version=$(VERSION)"
IDENTIFIER=$(GOOS)-$(GOARCH)

help:  ## Show available options with this Makefile
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -v grep | awk 'BEGIN { FS = ":.*?##" }; { printf "%-15s  %s\n", $$1,$$2 }'

web: dashboard
	@rm -rf web
	-@cd dashboard/web && yarn install && yarn build && cp -a ./dist ../../web

.PHONY: yarn
yarn:
	@rm -rf web
	@cd dashboard/web && yarn install && yarn build && cp -a ./dist ../../web

.PHONY: build
build: web
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GC) $(BUILD_NKND_PARAM) -o $(FLAGS)/nknd nknd.go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GC) $(BUILD_NKNC_PARAM) -o $(FLAGS)/nknc nknc.go

.PHONY: crossbuild
crossbuild: web
	mkdir -p build/$(IDENTIFIER)
	make build FLAGS="build/$(IDENTIFIER)"
	@cp -a dashboard/web/dist build/$(IDENTIFIER)/web

.PHONY: all
all: ## Build binaries for all available architectures
	@rm -rf web
	make crossbuild GOOS=linux GOARCH=arm
	make crossbuild GOOS=linux GOARCH=386
	make crossbuild GOOS=linux GOARCH=arm64
	make crossbuild GOOS=linux GOARCH=amd64
	make crossbuild GOOS=linux GOARCH=mips
	make crossbuild GOOS=linux GOARCH=mipsle
	make crossbuild GOOS=darwin GOARCH=amd64
	make crossbuild GOOS=darwin GOARCH=386
	make crossbuild GOOS=windows GOARCH=amd64
	make crossbuild GOOS=windows GOARCH=386

.PHONY: build_local
build_local: web ## Build local binaries without providing specific GOOS/GOARCH
	$(GC) $(BUILD_NKND_PARAM) nknd.go
	$(GC) $(BUILD_NKNC_PARAM) nknc.go
	## the following parts will be removed after the transition period from testnet to mainnet
	[ -s "wallet.dat" ] && [ -s "wallet.pswd" ] && ! [ -s "wallet.json" ] && cat wallet.pswd wallet.pswd | ./nknc wallet -c || :
	[ -s "config.json" ] && ! [ -s "config.json.bk" ] && grep -qE "022d52b07dff29ae6ee22295da2dc315fef1e2337de7ab6e51539d379aa35b9503|0149c42944eea91f094c16538eff0449d4d1e236f31c8c706b2e40e98402984c" config.json && mv config.json config.json.bk && cp config.mainnet.json config.json || :
	rm -f Chain/*.ldb

.PHONY: build_local_with_proxy
build_local_with_proxy: web ## Build local binaries with go proxy
	$(USE_PROXY) $(GC) $(BUILD_NKND_PARAM) nknd.go
	$(USE_PROXY) $(GC) $(BUILD_NKNC_PARAM) nknc.go

.PHONY: build_local_or_with_proxy
build_local_or_with_proxy:
	${MAKE} build_local || ${MAKE} build_local_with_proxy

.PHONY: format
format:  ## Run go format on nknd.go
	$(GOFMT) ./...

.PHONY: clean
clean:  ## Remove the nknd, nknc binaries and build directory
	rm -rf nknd nknc
	rm -rf build/

.PHONY: deepclean
deepclean:  ## Remove the existing binaries and build directory
	rm -rf nknd nknc build

.PHONY: pb
pb:
	protoc -I=. -I=$(GOPATH)/src -I=$(GOPATH)/src/github.com/gogo/protobuf/protobuf --gogoslick_out=. pb/*.proto
