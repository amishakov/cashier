CASHIER_CMD := ./cmd/cashier
CASHIERD_CMD := ./cmd/cashierd
SRC_FILES = $(shell find * -type f -name '*.go' -not -path 'vendor/*')
VERSION_PKG := github.com/cashier-go/cashier/lib.Version
VERSION := $(shell git describe --tags --always --dirty)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

ifeq ($(GOOS), linux)
  ifeq ($(CGO_ENABLED), 1)
    LINKER_FLAGS ?= -linkmode external -w -extldflags -static
  endif
endif

DOCKER_ARCHS := amd64 arm64 arm
BUILD_DOCKER_ARCHS = $(addprefix docker-,$(DOCKER_ARCHS))
TAG_DOCKER_ARCHS = $(addprefix docker-tag-latest-,$(DOCKER_ARCHS))

DOCKER_IMAGE_TAG ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))

.PHONY: all
all: build

.PHONY: test
test:
	# CGO is required to use the race detector
	CGO_ENABLED=1 go test -race ./...

.PHONY: build install
build: cashier cashierd
install: install-cashierd install-cashier

%-bin:
	CGO_ENABLED=$(CGO_ENABLED) GOARCH=$(GOARCH) GOOS=$(GOOS) go build -ldflags="-X $(VERSION_PKG)=$(VERSION) $(LINKER_FLAGS)" ./cmd/$*

install-%:
	CGO_ENABLED=$(CGO_ENABLED) GOARCH=$(GOARCH) GOOS=$(GOOS) go install -ldflags="-X $(VERSION_PKG)=$(VERSION) $(LINKER_FLAGS)" ./cmd/$*

.PHONY: docker-all-images $(BUILD_DOCKER_ARCHS)
docker-all-images: $(BUILD_DOCKER_ARCHS)
$(BUILD_DOCKER_ARCHS): docker-%:
	docker build --platform linux/$* -t linux-$*:$(DOCKER_IMAGE_TAG) .

.PHONY: docker-tag-all-latest $(TAG_DOCKER_ARCHS)
docker-tag-all-latest: $(TAG_DOCKER_ARCHS)
$(TAG_DOCKER_ARCHS): docker-tag-latest-%:
	docker tag "linux-$*:$(DOCKER_IMAGE_TAG)" "linux-$*:latest"

.PHONY: clean
clean:
	rm -f cashier cashierd

.PHONY: migration
# usage: make migration name=name_of_your_migration
# e.g. `make migration name=add_index_to_reason`
migration:
	go run ./generate/migration/migration.go $(name)

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: cashier cashierd
cashier: cashier-bin
cashierd: cashierd-bin

.PHONY: update-deps
update-deps:
	go get -u ./...
	go mod tidy

.PHONY: list-targets
list-targets:
	@LC_ALL=C $(MAKE) -pRrq -f $(lastword $(MAKEFILE_LIST)) : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]' -e '^$@$$'
