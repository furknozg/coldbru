.PHONY: test test-go test-unit test-web coverage generate generate-sqlc generate-openapi generate-graphql

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

# API_SPEC_SERVICES are the services that embed an OpenAPI spec (D-057); customer is GraphQL and has none.
API_SPEC_SERVICES := proposal notification analytics mockprovider

# Runs sqlc in every internal/ directory that holds a sqlc.yaml (D-051), then regenerates the
# OpenAPI 3.1 spec of each service in API_SPEC_SERVICES into cmd/<svc>/apidocs/swagger.json (D-057),
# and gqlgen the Customer GraphQL server code from its schema (D-058).
generate: generate-sqlc generate-openapi generate-graphql

generate-sqlc:
	@dirs=$$(find internal -name sqlc.yaml -exec dirname {} \;); \
	if [ -z "$$dirs" ]; then echo "generate: no sqlc.yaml found, nothing to do"; exit 0; fi; \
	for d in $$dirs; do echo "sqlc generate: $$d"; (cd "$$d" && go tool sqlc generate) || exit 1; done

# swag parses the service main package plus internal/platform/health (the shared platform endpoints).
# Add a service-specific handler package to --dir when it has annotations.
generate-openapi:
	@for s in $(API_SPEC_SERVICES); do \
	  echo "swag init: cmd/$$s"; \
	  mkdir -p cmd/$$s/apidocs; \
	  go tool swag init --quiet -g main.go -d cmd/$$s,internal/platform/health --parseInternal --v3.1 \
	    --outputTypes json -o cmd/$$s/apidocs || exit 1; \
	done

# gqlgen reads schema.graphqls and writes generated/, model/models_gen.go and the resolver
# stubs next to the config; paths in gqlgen.yml are relative to it, hence the cd (D-058).
generate-graphql:
	@echo "gqlgen generate: internal/customer/graph"
	@cd internal/customer/graph && go tool gqlgen generate --config gqlgen.yml
