.PHONY: run build test vet test-go test-scripts test-integration test-image tidy fixtures web-dev web-build docker

run:
	go run ./cmd/holodex

build:
	CGO_ENABLED=0 go build -tags production -o holodex ./cmd/holodex

test: test-go test-scripts

# The Go toolchain excludes any directory named `testdata` from package matching, so
# `./...` does not reach the stress-fixture seeder — it has to be named. Without this it
# is neither compiled nor tested by anything: it imports internal/repo, internal/mapping,
# internal/personimage, internal/imagesink and internal/thumbnail, so a signature change
# in any of them would merge green and only surface the next time someone tried to seed.
#
# Every Go target below uses this rather than `./...`, and CI calls those targets rather
# than restating the list — otherwise the next hidden package has to be remembered in
# four places, and forgetting one leaves CI green over code nothing compiles.
GO_PKGS := ./... ./testdata/stressseed

vet:
	go vet $(GO_PKGS)

test-go:
	go test $(GO_PKGS)

# Dependency-free node scripts (Jira sync, release-digest resolution, the worklog reader)
# plus the metadata-provider stub, whose /describe and /resolve have to keep agreeing about
# the namespace they issue ids in. Folded into `test` because
# scripts/resolve-release-digest.mjs decides what gets published as a release (ADR-070) —
# a green `make test` shouldn't be able to hide a break there.
test-scripts:
	node --test "scripts/**/*.test.mjs" "testdata/enrich-stub/**/*.test.mjs"

test-integration:
	go test -tags integration $(GO_PKGS)

# The packages that shell out to the media tools, tested INSIDE the built runtime image
# (ADR-117 D4), so ffmpeg, exiftool and MKVToolNix are the image's own versions, not the
# host's. It exists because a host-green run hid a real break: bookworm's exiftool 12.57
# couldn't read tags mkvpropedit had moved past the clusters. CI doesn't run integration
# tests (HOLODEX-487), so run this before any release that changes the Dockerfile or these
# packages. Each test binary is static and built for the image's OS; the source is mounted
# so the tests' relative paths resolve.
IMAGE_TEST_PKGS := writeback thumbnail metadata

test-image:
	docker build -t holodex:test-image .
	for p in $(IMAGE_TEST_PKGS); do \
		CGO_ENABLED=0 GOOS=linux go test -c -tags integration -o .test-image/$$p.test ./internal/$$p || exit 1; \
	done
	for p in $(IMAGE_TEST_PKGS); do \
		MSYS_NO_PATHCONV=1 docker run --rm --entrypoint /src/.test-image/$$p.test \
			-v "$(CURDIR):/src" -w /src/internal/$$p holodex:test-image -test.v || exit 1; \
	done

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
