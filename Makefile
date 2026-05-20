GOCACHE ?= $(PWD)/.gocache

.PHONY: test build frontend backend run

frontend:
	npm --prefix web install
	npm --prefix web run build

backend:
	GOCACHE=$(GOCACHE) go build -buildvcs=false -o bin/rbac-manager ./cmd/server

test:
	GOCACHE=$(GOCACHE) go test ./...
	npm --prefix web run build

build: frontend backend

run: build
	./bin/rbac-manager

