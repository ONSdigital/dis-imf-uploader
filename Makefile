BINPATH ?= build

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

BUILD_TIME=$(shell date +%s)
GIT_COMMIT=$(shell git rev-parse HEAD)
VERSION ?= $(shell git tag --points-at HEAD | grep ^v | head -n 1)

LDFLAGS = -ldflags "-X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT) -X main.Version=$(VERSION)"

.PHONY: all
all: delimiter-AUDIT audit delimiter-LINTERS lint delimiter-UNIT-TESTS test delimiter-FINISH ## Runs multiple targets, audit, lint, and test

.PHONY: audit
audit: ## Runs checks for security vulnerabilities on dependencies (including transient ones)
	dis-vulncheck

.PHONY: build
build: ## Builds binary of application code and stores in bin directory as dis-imf-uploader
	go build -tags 'production' $(LDFLAGS) -o $(BINPATH)/dis-imf-uploader

.PHONY: debug
debug: ## Used to run code locally in debug mode
	go build -tags 'debug' $(LDFLAGS) -o $(BINPATH)/dis-imf-uploader
	HUMAN_LOG=1 DEBUG=1 $(BINPATH)/dis-imf-uploader

.PHONY: debug-watch
debug-watch: ## Watches for changes and rebuilds in debug mode
	reflex -d none -c ./reflex

.PHONY: delimiter-%
delimiter-%:
	@echo '===================${GREEN} $* ${RESET}==================='

.PHONY: fmt
fmt: ## Formats Go code
	go fmt ./...

.PHONY: lint
lint: ## Runs Go linters
	golangci-lint run ./...

.PHONY: test
test: ## Runs unit tests with race condition checks
	go test -race -cover ./...

.PHONY: validate-specification
validate-specification: # Validate swagger spec
	redocly lint swagger.yaml

.PHONY: clean
clean: ## Removes build artifacts
	rm -rf $(BINPATH)

.PHONY: help
help: ## Show this help
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)