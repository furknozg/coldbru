// Package e2e holds the one cross-service end-to-end test (D-102). It has no production code: the
// test, under the integration build tag, wires the handlers and workers of every service in one
// process.
package e2e
