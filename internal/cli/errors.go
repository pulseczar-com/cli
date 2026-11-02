package cli

import (
	"errors"
	"fmt"

	"github.com/chrisggraham/pulseczar-cli/internal/api"
)

// Exit codes.
const (
	exitError = 1
	exitUsage = 2
	exitAuth  = 4
)

// Codes the CLI itself prints on the `code:` line, beside the API's.
const (
	codeUsage            = "usage"
	codeNotSignedIn      = "not_signed_in"
	codeAccessDenied     = "access_denied"
	codeExpiredToken     = "expired_token"
	codeLocalFailure     = "local_failure"
	codeUnexpectedAnswer = "unexpected_response"
)

// commandError is a failure with everything the CLI prints for it.
type commandError struct {
	exit    int
	code    string
	message string
	// details are extra lines printed after `code:`, such as a 422's field messages.
	details []string
}

func (e *commandError) Error() string { return e.message }

func usageError(format string, args ...any) error {
	return &commandError{exit: exitUsage, code: codeUsage, message: fmt.Sprintf(format, args...)}
}

func notSignedIn() error {
	return &commandError{
		exit:    exitAuth,
		code:    codeNotSignedIn,
		message: "not signed in",
		details: []string{"hint: run `pulseczar auth login`, or set PULSECZAR_TOKEN"},
	}
}

// fail turns any error a command met into a commandError.
func fail(err error) *commandError {
	var already *commandError
	if errors.As(err, &already) {
		return already
	}

	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		return fromAPI(apiErr)
	}

	var network *api.NetworkError
	if errors.As(err, &network) {
		return &commandError{exit: exitError, code: api.CodeNetwork, message: network.Error()}
	}

	return &commandError{exit: exitError, code: codeLocalFailure, message: err.Error()}
}

func fromAPI(err *api.Error) *commandError {
	failure := &commandError{exit: exitError, code: err.Code, message: err.Message}

	switch err.Code {
	case api.CodeUnauthenticated:
		failure.exit = exitAuth
		failure.details = []string{"hint: run `pulseczar auth login`"}
	case api.CodeForbidden:
		failure.exit = exitAuth
		failure.details = []string{"hint: this sign-in is read only; run `pulseczar auth login` and choose Read and write"}
	case api.CodeInvalid:
		failure.details = invalidDetails(err)
	}

	return failure
}

// invalidDetails lists a 422's messages by field.
func invalidDetails(err *api.Error) []string {
	var lines []string

	for _, field := range err.FieldNames() {
		for _, message := range err.Fields[field] {
			lines = append(lines, fmt.Sprintf("%s: %s", field, message))
		}
	}

	return lines
}

// reportError prints the error to stderr and returns the exit code. An error that
// is not a commandError came from the argument parser, so it is a usage error.
func (a *app) reportError(err error) int {
	var failure *commandError
	if !errors.As(err, &failure) {
		failure = &commandError{
			exit:    exitUsage,
			code:    codeUsage,
			message: err.Error(),
			details: []string{"hint: run `pulseczar --help`"},
		}
	}

	fmt.Fprintf(a.env.Stderr, "error: %s\ncode: %s\n", failure.message, failure.code)
	for _, line := range failure.details {
		fmt.Fprintln(a.env.Stderr, line)
	}

	return failure.exit
}
