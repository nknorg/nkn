.DEFAULT_GOAL:=build_local_or_with_proxy

GC=GO111MODULE=on go build
USE_PROXY=GOPROXY=https://goproxy.io
GOFMT=go fmt
VERSION:=$(shell git describe --abbrev=4 --dirty --always --tags)
Minversion:=$(shell date)
BUILD_NKND_PARAM=-ldflags "-s -w -X github.com/nknorg/nkn/util/config.Version=$(VERSION)"
BUILD_NKNC_PARAM=-ldflags "-s -w -X github.com/nknorg/nkn/cli/common.Version=$(VERSION)"
BUILD_DIR=build
BIN_DIR=$(GOOS)-$(GOARCH)

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
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GC) $(BUILD_NKND_PARAM) -o $(BUILD_DIR)/$(BIN_DIR)/nknd$(EXT) nknd.go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GC) $(BUILD_NKNC_PARAM) -o $(BUILD_DIR)/$(BIN_DIR)/nknc$(EXT) nknc.go

.PHONY: crossbuild
crossbuild: web
	mkdir -p $(BUILD_DIR)/$(BIN_DIR)
	${MAKE} build
	cp config.mainnet.json $(BUILD_DIR)/$(BIN_DIR)/default.json
	@cp -a dashboard/web/dist $(BUILD_DIR)/$(BIN_DIR)/web
ifeq ($(GOOS), windows)
	echo "IF NOT EXIST config.json COPY default.json config.json" > $(BUILD_DIR)/$(BIN_DIR)/start-gui.bat
	echo "nknd.exe --web-gui-create-wallet" >> $(BUILD_DIR)/$(BIN_DIR)/start-gui.bat
	chmod +x $(BUILD_DIR)/$(BIN_DIR)/start-gui.bat
endif
	${MAKE} zip

.PHONY: tar
tar:
	cd $(BUILD_DIR) && rm -f $(BIN_DIR).tar.gz && tar --exclude ".DS_Store" --exclude "__MACOSX" -czvf $(BIN_DIR).tar.gz $(BIN_DIR)

.PHONY: zip
zip:
	cd $(BUILD_DIR) && rm -f $(BIN_DIR).zip && zip --exclude "*.DS_Store*" --exclude "*__MACOSX*" -r $(BIN_DIR).zip $(BIN_DIR)

.PHONY: all
all: ## Build binaries for all available architectures
	@rm -rf web
	${MAKE} crossbuild GOOS=linux GOARCH=arm
	${MAKE} crossbuild GOOS=linux GOARCH=386
	${MAKE} crossbuild GOOS=linux GOARCH=arm64
	${MAKE} crossbuild GOOS=linux GOARCH=amd64
	${MAKE} crossbuild GOOS=linux GOARCH=mips
	${MAKE} crossbuild GOOS=linux GOARCH=mipsle
	${MAKE} crossbuild GOOS=darwin GOARCH=amd64
	${MAKE} crossbuild GOOS=darwin GOARCH=386
	${MAKE} crossbuild GOOS=windows GOARCH=amd64 EXT=.exe
	${MAKE} crossbuild GOOS=windows GOARCH=386 EXT=.exe

.PHONY: no_web
no_web:
	$(GC) $(BUILD_NKND_PARAM) nknd.go
	$(GC) $(BUILD_NKNC_PARAM) nknc.go

.PHONY: build_local
build_local: web ## Build local binaries without providing specific GOOS/GOARCH
	${MAKE} no_web
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
