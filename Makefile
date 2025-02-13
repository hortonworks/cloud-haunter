export BINARY=ch

GO_VERSION?=$(shell cat go.mod | grep '^go' | awk '{print $$2}')

BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
VERSION?=$(shell git describe --tags --abbrev=0)-snapshot
PKG_BASE=github.com/hortonworks/cloud-haunter
BUILD_TIME=$(shell date +%FT%T)
LDFLAGS=-w -s -X $(PKG_BASE)/context.Version=${VERSION} -X $(PKG_BASE)/context.BuildTime=${BUILD_TIME}

ifdef IGNORE_LABEL
LDFLAGS+= -X '$(PKG_BASE)/context.IgnoreLabel=$(IGNORE_LABEL)'
endif

ifdef OWNER_LABEL
LDFLAGS+= -X '$(PKG_BASE)/context.OwnerLabel=$(OWNER_LABEL)'
endif

ifdef RESOURCE_GROUPING_LABEL
LDFLAGS+= -X '$(PKG_BASE)/context.ResourceGroupingLabel=$(RESOURCE_GROUPING_LABEL)'
endif

ifdef RESOURCE_DESCRIPTION
LDFLAGS+= -X '$(PKG_BASE)/context.ResourceDescription=$(RESOURCE_DESCRIPTION)'
endif

ifdef AZURE_CREATION_TIME_LABEL
LDFLAGS+= -X '$(PKG_BASE)/context.AzureCreationTimeLabel=$(AZURE_CREATION_TIME_LABEL)'
endif

GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./.git/*")

all: deps build

deps:
ifeq (, $(shell which gh))
ifeq ($(shell uname),Linux)
	apt-get update
	apt-get -y install software-properties-common
	apt-key adv --keyserver keyserver.ubuntu.com --recv-key C99B11DEB97541F0
	apt-add-repository https://cli.github.com/packages
	apt update
	apt -y install gh
endif
ifeq ($(shell uname),Darwin)
	brew install gh
endif
	gh auth login
endif

_check: formatcheck vet

formatcheck:
	([ -z "$(shell gofmt -d $(GOFILES_NOVENDOR))" ]) || (echo "Source is unformatted"; exit 1)

format:
	gofmt -s -w $(GOFILES_NOVENDOR)

vet:
	GO111MODULE=on go vet -mod=vendor ./...

test:
	GO111MODULE=on go test -mod=vendor -timeout 30s -coverprofile coverage -race ./...

_build: build-darwin build-linux

build: format _check test _build

cleanup:
	rm -rf release && mkdir release

build-darwin:
	GO111MODULE=on GOOS=darwin CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o build/Darwin/${BINARY} main.go

build-linux:
	GO111MODULE=on GOOS=linux CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o build/Linux/${BINARY} main.go

build-docker:
	@#USER_NS='-u $(shell id -u $(whoami)):$(shell id -g $(whoami))'
	docker run --rm ${USER_NS} -v "${PWD}":/go/src/$(PKG_BASE) -w /go/src/$(PKG_BASE) golang:$(GO_VERSION) make build

mod-tidy:
	@#USER_NS='-u $(shell id -u $(whoami)):$(shell id -g $(whoami))'
	@docker run --rm ${USER_NS} -v "${PWD}":/go/src/$(PKG_BASE) -w /go/src/$(PKG_BASE) -e GO111MODULE=on golang:$(GO_VERSION) make _mod-tidy

_mod-tidy:
	go mod tidy -compat=$(GO_VERSION) -v
	go mod vendor

release: cleanup build
	./release.sh

download:
	./download.sh