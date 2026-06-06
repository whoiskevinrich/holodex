.PHONY: run build test test-integration tidy fixtures web-dev web-build docker

run:
	go run ./cmd/holodex

build:
	CGO_ENABLED=0 go build -tags production -o holodex ./cmd/holodex

test:
	go test ./...

test-integration:
	go test -tags integration ./...

tidy:
	go mod tidy

fixtures:
	./testdata/gen.sh

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

docker:
	docker compose up --build
