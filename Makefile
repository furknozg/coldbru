.PHONY: test test-go test-unit test-web coverage generate generate-sqlc generate-openapi generate-graphql generate-catalog generate-prompts consent

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
# gqlgen the Customer GraphQL server code from its schema (D-058), and the Proposal filter catalog
# from that schema plus its metadata (D-044, D-066), and the prompt lock file (D-059).
generate: generate-sqlc generate-openapi generate-graphql generate-catalog generate-prompts

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

# Writes internal/proposal/catalog/catalog_gen.go from the Customer schema and
# internal/proposal/catalog/metadata.yaml (D-044, D-066). Fails when the two disagree (a field
# without a description, a description without a field, a closed list on a non-String field).
# A freshness test fails when the committed file is stale.
generate-catalog:
	@echo "catalog gen: internal/proposal/catalog"
	@go run ./internal/proposal/catalog/gen

# Writes internal/proposal/prompts/prompts.lock.json: name, version and request-prefix sha256 of
# every prompt package (D-059). Fails when a prompt changed but its manifest version did not; the
# catalog is part of the segment prompt, so run generate-catalog first. A test fails when the
# committed lock is stale.
generate-prompts: generate-catalog
	@echo "prompts gen: internal/proposal/prompts"
	@go run ./internal/proposal/prompts/gen

# Developer command: records an opt-in or opt-out through Customer's updateConsent, as the admin
# caller, from inside the compose network (D-063, D-064). Docker must be on PATH and the stack up
# (`docker compose up -d customer`). Arguments reach the shell as environment variables, never
# spliced into the script, and are validated before the JSON body is built.
#   make consent CUSTOMER=CUS-000123 CHANNEL=sms OPTED_IN=false [SOURCE=admin_tool]
# CUSTOMER_TOKEN_ADMIN (default admin-dev-token) is read from the environment.
consent:
	@usage='usage: make consent CUSTOMER=CUS-000123 CHANNEL=<email|sms> OPTED_IN=<true|false> [SOURCE=admin_tool]'; \
	source="$${SOURCE:-admin_tool}"; \
	if ! printf '%s' "$$CUSTOMER" | grep -Eq '^CUS-[0-9]+$$'; then echo "consent: CUSTOMER must look like CUS-000123"; echo "$$usage"; exit 2; fi; \
	case "$$CHANNEL" in email|sms) ;; *) echo "consent: CHANNEL must be email or sms"; echo "$$usage"; exit 2;; esac; \
	case "$$OPTED_IN" in true|false) ;; *) echo "consent: OPTED_IN must be true or false"; echo "$$usage"; exit 2;; esac; \
	if ! printf '%s' "$$source" | grep -Eq '^[a-z_]+$$'; then echo "consent: SOURCE must be lower-case letters and underscores"; echo "$$usage"; exit 2; fi; \
	body=$$(printf '{"query":"mutation($$i: UpdateConsentInput!) { updateConsent(input: $$i) { customerId channel optedIn eligible } }","variables":{"i":{"customerId":"%s","channel":"%s","optedIn":%s,"source":"%s"}}}' "$$CUSTOMER" "$$CHANNEL" "$$OPTED_IN" "$$source"); \
	docker run --rm --network coldbru_default curlimages/curl:8.11.1 -sS -X POST http://customer:8080/graphql \
	  -H 'Content-Type: application/json' -H "Authorization: Bearer $${CUSTOMER_TOKEN_ADMIN:-admin-dev-token}" -d "$$body"; \
	echo
