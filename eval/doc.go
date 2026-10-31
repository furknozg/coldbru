// Package eval holds the prompt evaluation of D-115: hand-written cases under cases/ that run
// recorded model answers through the validation pipeline production uses, without an API key.
//
// The runner is eval_test.go behind the "eval" build tag, so `go test ./...` (make test) never
// picks it up; `make eval` does. See README.md in this directory for the case format.
package eval
