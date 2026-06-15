SHELL := /bin/sh

BIN_DIR ?= $(HOME)/.local/bin
SKILL_DIR ?= $(HOME)/.codex/skills
GUI_PORT ?= 0
GUI_HOST ?= 127.0.0.1

.PHONY: help test vet smoke check build gui install-local install-skill setup-check clean

help:
	@echo "web-tools local targets"
	@echo ""
	@echo "  make test          Run Go tests"
	@echo "  make vet           Run go vet"
	@echo "  make smoke         Run end-to-end smoke script"
	@echo "  make check         Run test, vet, and smoke"
	@echo "  make build         Build ./web-tools"
	@echo "  make gui           Start local GUI on $(GUI_HOST):$(GUI_PORT)"
	@echo "  make install-local Install CLI and skill to BIN_DIR/SKILL_DIR"
	@echo "  make install-skill Install or update only the local Agent skill"
	@echo "  make setup-check   Print setup readiness JSON"
	@echo "  make clean         Remove local build artifact"

test:
	@go test ./...

vet:
	@go vet ./...

smoke:
	@./scripts/smoke.sh

check: test vet smoke

build:
	@go build -o web-tools .

gui:
	@go run . gui --host "$(GUI_HOST)" --port "$(GUI_PORT)" --no-open

install-local:
	@BIN_DIR="$(BIN_DIR)" SKILL_DIR="$(SKILL_DIR)" sh scripts/install.sh

install-skill:
	@go run . skill install --dir "$(SKILL_DIR)" --source ./skills/web-tools/SKILL.md --force

setup-check:
	@go run . setup --check --json

clean:
	@rm -f web-tools
