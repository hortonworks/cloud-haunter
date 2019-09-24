BINARY=ch

BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
VERSION?=$(shell git describe --tags --abbrev=0)-snapshot
PKG_BASE=github.com/hortonworks/cloud-haunter
BUILD_TIME=$(shell date +%FT%T)
LDFLAGS=-w -s -X $(PKG_BASE)/context.Version=${VERSION} -X $(PKG_BASE)/context.BuildTime=${BUILD_TIME}

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

deps:
	go get -u github.com/keyki/glu

_check: formatcheck vet

formatcheck:
	([ -z "$(shell gofmt -d $(GOFILES_NOVENDOR))" ]) || (echo "Source is unformatted"; exit 1)

format:
	gofmt -s -w $(GOFILES_NOVENDOR)

vet:
	GO111MODULE=on go vet -mod=vendor ./...

test:
	GO111MODULE=on go test -mod=vendor -timeout 30s -coverprofile coverage -race

_build: build-darwin build-linux

build: _check test _build

cleanup:
	rm -rf release && mkdir release

build-darwin:
	GO111MODULE=on GOOS=darwin CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o build/Darwin/${BINARY} main.go

build-linux:
	GO111MODULE=on GOOS=linux CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o build/Linux/${BINARY} main.go

build-docker:
	@#USER_NS='-u $(shell id -u $(whoami)):$(shell id -g $(whoami))'
	docker run --rm ${USER_NS} -v "${PWD}":/go/src/$(PKG_BASE) -w /go/src/$(PKG_BASE) golang:1.13 make build

mod-tidy:
	@#USER_NS='-u $(shell id -u $(whoami)):$(shell id -g $(whoami))'
	@docker run --rm ${USER_NS} -v "${PWD}":/go/src/$(PKG_BASE) -w /go/src/$(PKG_BASE) -e GO111MODULE=on golang:1.13 make _mod-tidy

_mod-tidy:
	go mod tidy -v
	go mod vendor

release: cleanup build
	glu release

download:
	curl -LOs https://github.com/hortonworks/cloud-haunter/releases/download/v$(VERSION)/cloud-haunter_$(VERSION)_$(shell uname)_x86_64.tgz
	tar -xvf cloud-haunter_$(VERSION)_$(shell uname)_x86_64.tgz