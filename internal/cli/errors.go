package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/pulseczar-com/cli/internal/render"
	"github.com/spf13/cobra"
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
	// onTerminal replaces message and details when stderr is a terminal. The piped
	// form keeps the raw message, which scripts may match.
	onTerminal *terminalText
}

// terminalText is the plain wording of an error for a person.
type terminalText struct {
	message string
	hint    string
}

func (e *commandError) Error() string { return e.message }

func usageError(format string, args ...any) error {
	return &commandError{exit: exitUsage, code: codeUsage, message: fmt.Sprintf(format, args...)}
}

// signedOutText is what a person sees on a terminal whenever a command fails because they are signed out.
var signedOutText = &terminalText{
	message: "You are not signed in.",
	hint:    "Run `pulseczar auth login` to sign in.",
}

func notSignedIn() error {
	return &commandError{
		exit:       exitAuth,
		code:       codeNotSignedIn,
		message:    "not signed in",
		details:    []string{"hint: run `pulseczar auth login`, or set PULSECZAR_TOKEN"},
		onTerminal: signedOutText,
	}
}

// readOnlyText is what a person sees on a terminal when a read only sign-in tries to write.
var readOnlyText = &terminalText{
	message: "This sign-in is read only.",
	hint:    "Run `pulseczar auth login` and choose Read and write.",
}

// envTokenRejectedText is what a person sees when the server rejects the token in PULSECZAR_TOKEN.
// A new `auth login` cannot help, because the variable wins over the saved token.
var envTokenRejectedText = &terminalText{
	message: envToken + " was rejected.",
	hint:    "Unset it or replace it.",
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
		return &commandError{
			exit:       exitError,
			code:       api.CodeNetwork,
			message:    network.Error(),
			onTerminal: unreachable(network),
		}
	}

	return &commandError{exit: exitError, code: codeLocalFailure, message: err.Error()}
}

// unreachable words a network failure for a person: which server, and what to check.
func unreachable(network *api.NetworkError) *terminalText {
	server := "the server"

	var request *url.Error
	if errors.As(network.Err, &request) {
		if parsed, err := url.Parse(request.URL); err == nil && parsed.Hostname() != "" {
			server = parsed.Hostname()
		}
	}

	return &terminalText{
		message: "cannot reach " + server,
		hint:    "check your connection, or the address in " + envURL,
	}
}

func fromAPI(err *api.Error) *commandError {
	failure := &commandError{exit: exitError, code: err.Code, message: err.Message}

	switch err.Code {
	case api.CodeUnauthenticated:
		failure.exit = exitAuth
		failure.details = []string{"hint: run `pulseczar auth login`"}
		failure.onTerminal = signedOutText
	case api.CodeForbidden:
		failure.exit = exitAuth
		failure.details = []string{"hint: this sign-in is read only; run `pulseczar auth login` and choose Read and write"}
		failure.onTerminal = readOnlyText
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
		failure = parserError(err)
	}

	if a.env.StderrIsTerminal {
		a.printStyledError(failure)
	} else {
		a.printPlainError(failure)
	}

	return failure.exit
}

const suggestionIntro = "\n\nDid you mean this?\n"

// parserError turns a failure of cobra's parser into a usage error. Cobra's "Did you
// mean" list moves to the hint line, so the message stays on one line.
func parserError(err error) *commandError {
	message, suggestions, found := strings.Cut(err.Error(), suggestionIntro)

	hint := "hint: run `pulseczar --help`"
	if found {
		hint = "hint: did you mean `" + strings.Join(strings.Fields(suggestions), "` or `") + "`?"
	}

	return &commandError{exit: exitUsage, code: codeUsage, message: message, details: []string{hint}}
}

// printPlainError keeps the `error:` and `code:` lines that scripts read.
func (a *app) printPlainError(failure *commandError) {
	fmt.Fprintf(a.env.Stderr, "error: %s\ncode: %s\n", failure.message, failure.code)
	for _, line := range failure.details {
		fmt.Fprintln(a.env.Stderr, line)
	}
}

// terminalWording is the message and details a person sees for a failure.
func (a *app) terminalWording(failure *commandError) (string, []string) {
	text := failure.onTerminal
	if failure.code == api.CodeUnauthenticated && a.env.Getenv(envToken) != "" {
		text = envTokenRejectedText
	}
	if text == nil {
		return failure.message, failure.details
	}
	if text.hint == "" {
		return text.message, nil
	}

	return text.message, []string{text.hint}
}

// printStyledError is for a person: a red mark and the message, then each detail
// indented under it, with the "hint:" label dropped.
func (a *app) printStyledError(failure *commandError) {
	style := a.styleFor(true)
	message, details := a.terminalWording(failure)

	fmt.Fprintf(a.env.Stderr, "%s %s\n", style.Paint(render.Red, render.MarkFailed), message)
	for _, line := range details {
		fmt.Fprintf(a.env.Stderr, "  %s\n", strings.TrimPrefix(line, "hint: "))
	}
}

// exactlyOneID checks the argument count with plain words, in place of cobra's
// "accepts 1 arg(s), received 0".
func exactlyOneID(what, listCommand string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		switch {
		case len(args) == 0:
			return usageError("give %s; run `%s` to see them", what, listCommand)
		case len(args) > 1:
			return usageError("give only one id, not %d", len(args))
		default:
			return nil
		}
	}
}
