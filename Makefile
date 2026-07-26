.PHONY: run build test test-go test-scripts test-integration tidy fixtures web-dev web-build docker

run:
	go run ./cmd/holodex

build:
	CGO_ENABLED=0 go build -tags production -o holodex ./cmd/holodex

test: test-go test-scripts

test-go:
	go test ./...

# Dependency-free node scripts (Jira sync, release-digest resolution) plus the Flightplan
# library. Folded into `test` because scripts/resolve-release-digest.mjs decides what gets
# published as a release (ADR-070) — a green `make test` shouldn't be able to hide a break
# there — and because flightplan/lib/worklog.mjs *writes* to worklogs from a hook.
test-scripts:
	node --test "scripts/**/*.test.mjs" "flightplan/**/*.test.mjs"

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
