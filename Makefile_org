.DEFAULT_GOAL:=build_local_or_with_proxy

GC=GO111MODULE=on go build
GO_PROXY=https://goproxy.io
GOFMT=go fmt
VERSION:=$(shell git describe --abbrev=4 --dirty --always --tags)
BUILD_DIR=build
ifdef GOARM
BIN_DIR=$(GOOS)-$(GOARCH)v$(GOARM)
else
BIN_DIR=$(GOOS)-$(GOARCH)
endif
NKND_BUILD_PARAM=-ldflags "-s -w -X github.com/nknorg/nkn/v2/config.Version=$(VERSION)"
#NKNC_BUILD_PARAM=-ldflags "-s -w -X github.com/nknorg/nkn/v2/cmd/nknc/common.Version=$(VERSION)"
NKNC_BUILD_PARAM=$(NKND_BUILD_PARAM)
NKND_OUTPUT=$(BUILD_DIR)/$(BIN_DIR)/nknd$(EXT)
NKNC_OUTPUT=$(BUILD_DIR)/$(BIN_DIR)/nknc$(EXT)
NKND_MAIN=./cmd/nknd/
NKNC_MAIN=./cmd/nknc/

help:  ## Show available options with this Makefile
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -v grep | awk 'BEGIN { FS = ":.*?##" }; { printf "%-15s  %s\n", $$1,$$2 }'

web: $(shell find dashboard/web -type f -not -path "dashboard/web/node_modules/*" -not -path "dashboard/web/dist/*" -not -path "dashboard/web/.nuxt/*")
	-@cd dashboard/web && yarn install && yarn build && rm -rf ../../web && cp -a ./dist ../../web

.PHONY: build
build: web
	GOPROXY=$(GOPROXY) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) $(GC) $(NKND_BUILD_PARAM) -o $(NKND_OUTPUT) $(NKND_MAIN)
	GOPROXY=$(GOPROXY) GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) $(GC) $(NKNC_BUILD_PARAM) -o $(NKNC_OUTPUT) $(NKNC_MAIN)

.PHONY: crossbuild
crossbuild: web
	rm -rf $(BUILD_DIR)/$(BIN_DIR)
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
	${MAKE} crossbuild GOOS=linux GOARCH=arm GOARM=5
	${MAKE} crossbuild GOOS=linux GOARCH=arm GOARM=6
	${MAKE} crossbuild GOOS=linux GOARCH=arm GOARM=7
	${MAKE} crossbuild GOOS=linux GOARCH=386
	${MAKE} crossbuild GOOS=linux GOARCH=arm64
	${MAKE} crossbuild GOOS=linux GOARCH=amd64
	${MAKE} crossbuild GOOS=linux GOARCH=mips
	${MAKE} crossbuild GOOS=linux GOARCH=mipsle
	${MAKE} crossbuild GOOS=darwin GOARCH=amd64
	${MAKE} crossbuild GOOS=darwin GOARCH=arm64
	${MAKE} crossbuild GOOS=windows GOARCH=amd64 EXT=.exe
	${MAKE} crossbuild GOOS=windows GOARCH=386 EXT=.exe

.PHONY: build_local
build_local: web
	${MAKE} build BUILD_DIR=. BIN_DIR=.

.PHONY: build_local_with_proxy
build_local_with_proxy: web
	${MAKE} build_local GOPROXY=$(GO_PROXY)

.PHONY: build_local_or_with_proxy
build_local_or_with_proxy:
	${MAKE} build_local || ${MAKE} build_local_with_proxy

.PHONY: format
format:
	$(GOFMT) ./...

.PHONY: clean
clean:
	rm -rf nknd nknc
	rm -rf build/

.PHONY: deepclean
deepclean:
	rm -rf nknd nknc build

.PHONY: pb
pb:
	protoc --go_out=. pb/*.proto

.PHONY: test
test:
	go test -v ./chain/store

.PHONY: docker
docker:
	docker build -f docker/Dockerfile -t nknorg/nkn:latest-amd64 .
	docker build -f docker/Dockerfile --build-arg build_args="build GOOS=linux GOARCH=arm GOARM=6 BUILD_DIR=. BIN_DIR=." --build-arg base="arm32v6/" -t nknorg/nkn:latest-arm32v6 .
	docker build -f docker/Dockerfile --build-arg build_args="build GOOS=linux GOARCH=arm64 BUILD_DIR=. BIN_DIR=." --build-arg base="arm64v8/" -t nknorg/nkn:latest-arm64v8 .

.PHONY: docker_publish
docker_publish:
	docker push nknorg/nkn:latest-amd64
	docker push nknorg/nkn:latest-arm32v6
	docker push nknorg/nkn:latest-arm64v8
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create nknorg/nkn:latest nknorg/nkn:latest-amd64 nknorg/nkn:latest-arm32v6 nknorg/nkn:latest-arm64v8 --amend
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest annotate nknorg/nkn:latest nknorg/nkn:latest-arm32v6 --os linux --arch arm
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest annotate nknorg/nkn:latest nknorg/nkn:latest-arm64v8 --os linux --arch arm64
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest push -p nknorg/nkn:latest
