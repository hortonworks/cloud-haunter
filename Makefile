export BINARY=ch

# This version refers to the next release version required,
# which will be increased automatically by the dedicated release job
export VERSION=0.5.72
GO_VERSION?=$(shell cat go.mod | grep '^go' | awk '{print $$2}')
PKG_BASE=github.com/hortonworks/cloud-haunter
BUILD_TIME=$(shell date +%FT%T)
LDFLAGS=-w -s -X $(PKG_BASE)/context.Version=${VERSION} -X $(PKG_BASE)/context.BuildTime=${BUILD_TIME}

GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./.git/*")
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

deps:
ifeq ($(shell uname),Linux)
ifeq (, $(shell which gh))
	apt-get update
	apt-get -y install software-properties-common
	apt-key adv --keyserver keyserver.ubuntu.com --recv-key 23F3D4EA75716059
	apt-add-repository https://cli.github.com/packages
	apt update
	apt -y install gh
endif
ifeq (, $(shell which aws))
	apt-get update
	apt-get -y install awscli
endif
endif

formatcheck:
	([ -z "$(shell gofmt -d $(GOFILES_NOVENDOR))" ]) || (echo "Source is unformatted"; exit 1)

format:
	@gofmt -w ${GOFILES_NOVENDOR}

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
	docker run --rm ${USER_NS} -v "${PWD}":/go/src/github.com/hortonworks/cloud-haunter -w /go/src/github.com/hortonworks/cloud-haunter -e VERSION=${VERSION} golang:$(GO_VERSION) make build

install: build ## Installs OS specific binary into: /usr/local/bin
	install build/$(shell uname -s)/$(BINARY) /usr/local/bin

release: 
	make build
	./release.sh

release-docker:	@USER_NS='-u$(shell id -u $(whoami)):$(shell id -g $(whoami))'
release-docker:
	sleep 30 ## wait for docker service on jenkins slave
	docker run --rm ${USER_NS} -v "${PWD}":/go/src/github.com/hortonworks/cloud-haunter -w /go/src/github.com/hortonworks/cloud-haunter -e VERSION=${VERSION} -e GITHUB_TOKEN=${GITHUB_TOKEN} -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} -e GO111MODULE=on golang:$(GO_VERSION) make deps release

gitPush:
	@if ! git diff-index --quiet HEAD Makefile; then\
		git add Makefile;\
		git commit -m "Increase cloud haunter version";\
		git push origin HEAD:$(GIT_BRANCH);\
	else \
		echo No changes Makefile, no git push needed.;\
	fi

mod-tidy:
	@docker run --rm -v "${PWD}":/go/src/github.com/hortonworks/cloud-haunter -w /go/src/github.com/hortonworks/cloud-haunter golang:$(GO_VERSION) make _mod-tidy

_mod-tidy:
	GO111MODULE=on go mod tidy -v
	GO111MODULE=on go mod vendor

.PHONY: build
