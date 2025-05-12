PACKAGE = github.com/Loragon-chain/loragon-consensus

GIT_COMMIT = $(shell git --no-pager log --pretty="%h" -n 1)
GIT_TAG = $(shell git tag -l --points-at HEAD)
METER_VERSION = $(shell cat cmd/loragon/VERSION)
DISCO_VERSION = $(shell cat cmd/disco/VERSION)

PACKAGES = `go list ./... | grep -v '/vendor/'`

MAJOR = $(shell go version | cut -d' ' -f3 | cut -b 3- | cut -d. -f1)
MINOR = $(shell go version | cut -d' ' -f3 | cut -b 3- | cut -d. -f2)
export GO111MODULE=on

.PHONY: loragon disco mdb all clean test

loragon:| go_version_check
	@echo "building $@..."
	@go build -v -o $(CURDIR)/bin/$@ -ldflags "-X main.version=$(METER_VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)"  -tags '$(BUILD_TAGS)' ./cmd/loragon
	@echo "done. executable created at 'bin/$@'"

mdb:| go_version_check
	@echo "building $@..."
	@go build -v -o $(CURDIR)/bin/$@ -ldflags "-X main.version=$(METER_VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)" ./cmd/mdb
	@echo "done. executable created at 'bin/$@'"


disco:| go_version_check
	@echo "building $@..."
	@go build -v -o $(CURDIR)/bin/$@ -ldflags "-X main.version=$(DISCO_VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)" ./cmd/disco
	@echo "done. executable created at 'bin/$@'"

dep:| go_version_check
	@go mod download

go_version_check:
	@if test $(MAJOR) -lt 1; then \
		echo "Go 1.13 or higher required"; \
		exit 1; \
	else \
		if test $(MAJOR) -eq 1 -a $(MINOR) -lt 13; then \
			echo "Go 1.13 or higher required"; \
			exit 1; \
		fi \
	fi

all: loragon disco mdb

clean:
	-rm -rf \
$(CURDIR)/bin/loragon \
$(CURDIR)/bin/disco 

test:| go_version_check
	@go test -cover $(PACKAGES)

build:
	go build -o loragon -tags bls12381 main.go sample_app.go code.go
