export BINARY=ch

# This version refers to the next release version required,
# which will be increased automatically by the dedicated release job
export VERSION=0.5.72
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
	GO111MODULE=on go test -mod=vendor -timeout 30s -race ./...

build: vet formatcheck test build-darwin build-linux

build-darwin:
	GOOS=darwin GO111MODULE=on go build -a -installsuffix cgo ${LDFLAGS} -o build/Darwin/${BINARY} main.go

build-linux:
	GOOS=linux GO111MODULE=on go build -a -installsuffix cgo ${LDFLAGS} -o build/Linux/${BINARY} main.go

build-docker:
	@#USER_NS='-u $(shell id -u $(whoami)):$(shell id -g $(whoami))'
	docker run --rm ${USER_NS} -v "${PWD}":/go/src/github.com/hortonworks/cloud-haunter -w /go/src/github.com/hortonworks/cloud-haunter -e VERSION=${VERSION} golang:1.23.12 make build

install: build ## Installs OS specific binary into: /usr/local/bin
	install build/$(shell uname -s)/$(BINARY) /usr/local/bin

release: 
	make build
	# ./release.sh

release-docker:	@USER_NS='-u$(shell id -u $(whoami)):$(shell id -g $(whoami))'
release-docker:
	sleep 60 ## wait for docker service on jenkins slave
	docker run --rm ${USER_NS} -v "${PWD}":/go/src/github.com/hortonworks/cloud-haunter -w /go/src/github.com/hortonworks/cloud-haunter -e VERSION=${VERSION} -e GITHUB_TOKEN=${GITHUB_TOKEN} -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} -e GO111MODULE=on golang:1.17 make deps release

gitPush:
	@if ! git diff-index --quiet HEAD Makefile; then\
		git add Makefile;\
		git commit -m "Increase image catalog cli version";\
		git push origin HEAD:$(GIT_BRANCH);\
	else \
		echo No changes Makefile, no git push needed.;\
	fi

mod-tidy:
	@docker run --rm -v "${PWD}":/go/src/github.com/hortonworks/cloud-haunter -w /go/src/github.com/hortonworks/cloud-haunter golang:1.23.12 make _mod-tidy

_mod-tidy:
	GO111MODULE=on go mod tidy -v
	GO111MODULE=on go mod vendor

.PHONY: build
