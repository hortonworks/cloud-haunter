export BINARY=ch

# This version refers to the next release version required,
# which will be increased automatically by the dedicated release job
export VERSION=0.5.75

ifeq ($(GH_PRE_RELEASE),true)
	OLD_VER:=$(VERSION)
    export VERSION=$(OLD_VER)-dev$(shell date +%s.%N)
endif

GO_VERSION?=$(shell cat go.mod | grep '^go' | awk '{print $$2}')
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
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
GOLANG_CONTAINER?=docker-private.infra.cloudera.com/cloudera_thirdparty/golang

deps:
ifeq ($(shell uname),Linux)
ifeq (, $(shell which gh))
# Based on the official GH CLI docs: https://github.com/cli/cli/blob/trunk/docs/install_linux.md#debian
# Snapshot: https://web.archive.org/web/20260710093431/https://github.com/cli/cli/blob/trunk/docs/install_linux.md#debian
	(type -p wget >/dev/null || (apt update && apt install wget -y)) \
	&& mkdir -p /etc/apt/keyrings \
	&& wget -nv -O /etc/apt/keyrings/githubcli-archive-keyring.gpg https://cli.github.com/packages/githubcli-archive-keyring.gpg \
	&& mkdir -p /etc/apt/sources.list.d \
	&& echo "deb [arch=$$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list \
	&& apt update \
	&& apt install gh -y
endif
endif

formatcheck:
	([ -z "$(shell gofmt -d $(GOFILES_NOVENDOR))" ]) || (echo "Source is unformatted"; exit 1)

format:
	@gofmt -s -w ${GOFILES_NOVENDOR}

vet:
	GO111MODULE=on go vet -mod=vendor ./...

test:
	GO111MODULE=on go test -mod=vendor -timeout 30s -coverprofile coverage -race ./...

build: vet formatcheck test build-darwin build-linux

build-darwin:
	GO111MODULE=on GOOS=darwin CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o build/Darwin/${BINARY} main.go

build-linux:
	GO111MODULE=on GOOS=linux CGO_ENABLED=0 go build -mod=vendor -ldflags "$(LDFLAGS)" -o build/Linux/${BINARY} main.go

build-docker:
	@#USER_NS='-u $(shell id -u $(whoami)):$(shell id -g $(whoami))'
	docker run --rm ${USER_NS} -v "${PWD}":/go/src/github.com/hortonworks/cloud-haunter \
	-w /go/src/github.com/hortonworks/cloud-haunter \
	-e VERSION=${VERSION} \
	-e GH_PRE_RELEASE=${GH_PRE_RELEASE} \
	$(GOLANG_CONTAINER):$(GO_VERSION) make build

install: build ## Installs OS specific binary into: /usr/local/bin
	install build/$(shell uname -s)/$(BINARY) /usr/local/bin

release: 
	make build
	./release.sh

release-docker:
	@USER_NS='-u$(shell id -u $(whoami)):$(shell id -g $(whoami))'
	sleep 30 ## wait for docker service on jenkins slave
	docker run --rm ${USER_NS} \
	-v "${PWD}":/go/src/github.com/hortonworks/cloud-haunter \
	-w /go/src/github.com/hortonworks/cloud-haunter \
	-e VERSION=${VERSION} -e GITHUB_TOKEN=${GITHUB_TOKEN} \
	-e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
	-e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
	-e GO111MODULE=on \
	-e GH_PRE_RELEASE=${GH_PRE_RELEASE} \
	$(GOLANG_CONTAINER):$(GO_VERSION) make deps release

gitPush:
	@if ! git diff-index --quiet HEAD Makefile; then\
		git add Makefile;\
		git commit -m "Increase cloud haunter version";\
		git push origin HEAD:$(GIT_BRANCH);\
	else \
		echo No changes Makefile, no git push needed.;\
	fi

mod-tidy:
	@#USER_NS='-u $(shell id -u $(whoami)):$(shell id -g $(whoami))'
	@docker run --rm ${USER_NS} \
	-v "${PWD}":/go/src/github.com/hortonworks/cloud-haunter \
	-w /go/src/github.com/hortonworks/cloud-haunter \
	$(GOLANG_CONTAINER):$(GO_VERSION) make _mod-tidy

_mod-tidy:
	GO111MODULE=on go mod tidy -compat=$(GO_VERSION) -v
	GO111MODULE=on go mod vendor

download:
	./download.sh

.PHONY: build
