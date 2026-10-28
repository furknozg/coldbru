package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"
)

// decode reads the JSON body of r into dst: the media type must be application/json, the body at
// most MaxBodyBytes, a single JSON value, and no unknown fields. On failure it has answered the
// request and returns false.
//
//	415 the media type is not JSON;  413 the body is too large;  400 the JSON is malformed or
//	empty;  422 the JSON does not fit the request type (an unknown field, a wrong type).
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		fail(w, r, CodeUnsupportedMedia, "The request body must be sent as application/json.")
		return false
	}
	body := http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		bodyError(w, r, err)
		return false
	}
	// Only one value is allowed; anything but white space after it is an error.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			fail(w, r, CodeMalformedJSON, "The request body holds more than one JSON value.")
		} else {
			bodyError(w, r, err)
		}
		return false
	}
	return true
}

// bodyError answers a decoding error.
func bodyError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		tooLarge *http.MaxBytesError
		syntax   *json.SyntaxError
		typ      *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &tooLarge):
		fail(w, r, CodePayloadTooLarge, fmt.Sprintf("The request body is larger than %d bytes.", MaxBodyBytes))
	case errors.Is(err, io.EOF):
		fail(w, r, CodeMalformedJSON, "The request body is empty.")
	case errors.Is(err, io.ErrUnexpectedEOF), errors.As(err, &syntax):
		fail(w, r, CodeMalformedJSON, "The request body is not valid JSON.")
	case errors.As(err, &typ):
		field := typ.Field
		if field == "" {
			field = "body"
		}
		fail(w, r, CodeInvalidRequest, "The request body does not match the expected shape.", fmt.Sprintf("%s: expected %s", field, typ.Type))
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		fail(w, r, CodeInvalidRequest, "The request body does not match the expected shape.", strings.TrimPrefix(err.Error(), "json: "))
	default:
		fail(w, r, CodeMalformedJSON, "The request body could not be read as JSON.")
	}
}

// checkText validates a required text field and answers a 422 problem when it is empty or longer
// than max characters. It returns the trimmed text.
func checkText(w http.ResponseWriter, r *http.Request, field, value string, max int) (string, bool) {
	value = strings.TrimSpace(value)
	switch n := utf8.RuneCountInString(value); {
	case n == 0:
		fail(w, r, CodeInvalidRequest, "The request body does not match the expected shape.", field+": must not be empty")
		return "", false
	case n > max:
		fail(w, r, CodeInvalidRequest, "The request body does not match the expected shape.", fmt.Sprintf("%s: must be at most %d characters (got %d)", field, max, n))
		return "", false
	}
	return value, true
}
