VERSION=$(patsubst "%",%,$(lastword $(shell grep "version\s*=\s" version.go)))
BIN_DIR=bin
BUILD_GOLANG_VERSION=1.8.1

.PHONY : build-with-docker
build-with-docker:
	docker run --rm -v "$(PWD)":/go/src/github.com/wyukawa/fluentd_exporter -w /go/src/github.com/wyukawa/fluentd_exporter golang:$(BUILD_GOLANG_VERSION) bash -c 'make build-all'

.PHONY : build-all
build-all: build-mac build-linux

.PHONY : build-mac
build-mac:
	make build GOOS=darwin GOARCH=amd64

.PHONY : build-linux
build-linux:
	make build GOOS=linux GOARCH=amd64

build:
	rm -rf $(BIN_DIR)/fluentd_exporter-$(VERSION).$(GOOS)-$(GOARCH)*
	go build -o $(BIN_DIR)/fluentd_exporter-$(VERSION).$(GOOS)-$(GOARCH)/fluentd_exporter
	tar cvfz $(BIN_DIR)/fluentd_exporter-$(VERSION).$(GOOS)-$(GOARCH).tar.gz -C $(BIN_DIR) fluentd_exporter-$(VERSION).$(GOOS)-$(GOARCH)
