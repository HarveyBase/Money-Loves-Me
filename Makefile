.PHONY: build run test test-verbose clean migrate web-install web-build

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -count=1 -timeout 180s

test-verbose:
	go test ./... -count=1 -v -timeout 180s

clean:
	rm -rf bin/ web/node_modules web/dist

migrate:
	@echo "Run migrations/001_init.sql against your MySQL database"

web-install:
	cd web && npm install

web-build:
	cd web && npx tsc --noEmit && npx vite build

all: build web-build
