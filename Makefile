SHA := $(shell git rev-parse --short=8 HEAD)
GITVERSION := $(shell git describe --long --all)
BUILDDATE := $(shell date -Iseconds)
VERSION := $(or ${VERSION},$(shell git describe --tags --exact-match 2> /dev/null || git symbolic-ref -q --short HEAD || git rev-parse --short HEAD))

CGO_ENABLED := 1
LINKMODE := -extldflags '-static -s -w'

ifeq ($(CI),true)
  GO_TEST_ARGS=-p 1 -count=1
else
  GO_TEST_ARGS=
endif

all: fmt test token-refresher

.PHONY: token-refresher
token-refresher: fmt
	go build -tags netgo,osusergo,urfave_cli_no_docs \
		 -ldflags "$(LINKMODE) -X 'github.com/metal-stack/v.Version=$(VERSION)' \
								   -X 'github.com/metal-stack/v.Revision=$(GITVERSION)' \
								   -X 'github.com/metal-stack/v.GitSHA1=$(SHA)' \
								   -X 'github.com/metal-stack/v.BuildDate=$(BUILDDATE)'" \
	   -o bin/token-refresher github.com/metal-stack/metal-token-refresher/cmd/token-refresher
	strip bin/token-refresher

.PHONY: test
test:
	go test ./... -race -coverpkg=./... -coverprofile=coverage.out -covermode=atomic $(GO_TEST_ARGS) -timeout=300s && go tool cover -func=coverage.out

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: golint
golint:
	golangci-lint run -p bugs -p unused -D protogetter
