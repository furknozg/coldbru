.PHONY: test test-go test-unit test-web coverage generate

# testcontainers-go needs the Docker socket. Docker Desktop on macOS keeps it under
# ~/.docker/run; point DOCKER_HOST there only if it exists and the caller set nothing.
ifeq ($(origin DOCKER_HOST),undefined)
ifneq ($(wildcard $(HOME)/.docker/run/docker.sock),)
export DOCKER_HOST := unix://$(HOME)/.docker/run/docker.sock
endif
endif

# Every suite: Go unit + integration (needs Docker) and web (D-051).
test: test-go test-web

# Go unit and integration tests; integration files carry the "integration" build tag (D-051).
test-go:
	go test -race -count=1 -tags=integration ./...

# Unit tests only, no Docker needed (D-051).
test-unit:
	go test -race -count=1 ./...
	$(MAKE) test-web

test-web:
	pnpm -C web test:unit --run

# Unit test coverage, reported to coverage/ (gitignored); no thresholds (D-052).
coverage:
	mkdir -p coverage
	go test -count=1 -covermode=atomic -coverprofile=coverage/go.out ./...
	go tool cover -func=coverage/go.out | tail -n 1
	go tool cover -html=coverage/go.out -o coverage/go.html
	pnpm -C web test:unit --run --coverage

# Runs sqlc in every internal/ directory that holds a sqlc.yaml (D-051).
generate:
	@dirs=$$(find internal -name sqlc.yaml -exec dirname {} \;); \
	if [ -z "$$dirs" ]; then echo "generate: no sqlc.yaml found, nothing to do"; exit 0; fi; \
	for d in $$dirs; do echo "sqlc generate: $$d"; (cd "$$d" && go tool sqlc generate) || exit 1; done
