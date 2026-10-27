.PHONY: test test-go test-web

# Runs every test suite. Integration tests (D-025) will join once services exist.
test: test-go test-web

# go test exits non-zero when no packages exist yet (D-029), so skip until there is Go code.
test-go:
	@if [ -n "$$(go list ./... 2>/dev/null)" ]; then go test ./...; else echo "test-go: no Go packages yet, skipping"; fi

test-web:
	pnpm -C web test:unit --run
