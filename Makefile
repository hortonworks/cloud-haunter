BINARY=ccr

BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
VERSION?=$(shell git describe --tags --abbrev=0)-snapshot
PKG_BASE=github.com/hortonworks/cloud-cost-reducer
BUILD_TIME=$(shell date +%FT%T)
LDFLAGS=-X $(PKG_BASE)/context.Version=${VERSION} -X $(PKG_BASE)/context.BuildTime=${BUILD_TIME}

AWS_IGNORE_LABEL?=cloud-cost-reducer-ignore
LDFLAGS+= -X $(PKG_BASE)/context.AwsIgnoreLabel=$(AWS_IGNORE_LABEL)
AWS_OWNER_LABEL?=Owner
LDFLAGS+= -X $(PKG_BASE)/contextgcp.AwsOwnerLabel=$(AWS_OWNER_LABEL)

AZURE_IGNORE_LABEL?=cloud-cost-reducer-ignore
LDFLAGS+= -X $(PKG_BASE)/context.AzureIgnoreLabel=$(AZURE_IGNORE_LABEL)
AZURE_OWNER_LABEL?=Owner
LDFLAGS+= -X $(PKG_BASE)/contextgcp.AzureOwnerLabel=$(AZURE_OWNER_LABEL)

GCP_IGNORE_LABEL?=cloud-cost-reducer-ignore
LDFLAGS+= -X $(PKG_BASE)/context.GcpIgnoreLabel=$(GCP_IGNORE_LABEL)
GCP_OWNER_LABEL?=owner
LDFLAGS+= -X $(PKG_BASE)/contextgcp.GcpOwnerLabel=$(GCP_OWNER_LABEL)

GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./.git/*")

all: deps build

deps: deps-errcheck
	go get -u github.com/golang/dep/cmd/dep
	# go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/keyki/gh-release/...

deps-errcheck:
	go get -u github.com/kisielk/errcheck

format:
	@gofmt -w ${GOFILES_NOVENDOR}

vet:
	go vet ./...

test:
	go test -timeout 30s -race ./...

errcheck:
	errcheck -ignoretests ./...

build: errcheck format vet test build-darwin build-linux

build-docker:
	@#USER_NS='-u $(shell id -u $(whoami)):$(shell id -g $(whoami))'
	docker run --rm ${USER_NS} -v "${PWD}":/go/src/$(PKG_BASE) -w /go/src/$(PKG_BASE) golang:1.9 make deps-errcheck build

build-darwin:
	GOOS=darwin CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o build/Darwin/${BINARY} main.go

build-linux:
	GOOS=linux CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o build/Linux/${BINARY} main.go

release: build
	rm -rf release && mkdir release
	tar -zcf release/$(BINARY)_$(VERSION)_Linux_$(shell uname -m).tgz -C build/Linux $(BINARY)
	tar -zcf release/$(BINARY)_$(VERSION)_Darwin_$(shell uname -m).tgz -C build/Darwin $(BINARY)
	gh-release create hortonworks/cloud-cost-reducer $(VERSION) $(BRANCH) v$(VERSION)

download:
	curl -LOs https://github.com/hortonworks/cloud-cost-reducer/releases/download/v$(VERSION)/ccr_$(VERSION)_$(shell uname)_x86_64.tgz
	tar -xvf ccr_$(VERSION)_$(shell uname)_x86_64.tgz