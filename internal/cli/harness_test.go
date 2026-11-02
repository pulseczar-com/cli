package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const testToken = "1|test-token"

// request is what the fake Czar saw.
type request struct {
	Method string
	Path   string
	Query  url.Values
	Auth   string
	Body   map[string]any
}

// reply is what a fake route answers: a status and a fixture file.
type reply struct {
	status  int
	fixture string
}

// fakeCzar is an httptest server standing in for Czar. Routes are "METHOD /path".
// It records every request, and answers 404 JSON to a route nobody set up.
type fakeCzar struct {
	t        *testing.T
	server   *httptest.Server
	routes   map[string][]reply
	requests []request
}

func newFakeCzar(t *testing.T, routes map[string][]reply) *fakeCzar {
	t.Helper()

	czar := &fakeCzar{t: t, routes: routes}
	czar.server = httptest.NewServer(http.HandlerFunc(czar.serve))
	t.Cleanup(czar.server.Close)

	return czar
}

func (c *fakeCzar) serve(w http.ResponseWriter, r *http.Request) {
	seen := request{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Auth: r.Header.Get("Authorization")}
	_ = json.NewDecoder(r.Body).Decode(&seen.Body)
	c.requests = append(c.requests, seen)

	key := r.Method + " " + r.URL.Path
	replies := c.routes[key]
	if len(replies) == 0 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not found."}`))

		return
	}

	// The last reply repeats, so a route that answers once needs one entry.
	answer := replies[0]
	if len(replies) > 1 {
		c.routes[key] = replies[1:]
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(answer.status)
	if answer.fixture != "" {
		w.Write(readFixture(c.t, answer.fixture))
	}
}

func (c *fakeCzar) only() request {
	c.t.Helper()
	if len(c.requests) != 1 {
		c.t.Fatalf("want 1 request, got %d: %+v", len(c.requests), c.requests)
	}

	return c.requests[0]
}

// withStableURL swaps the test server's random port for a fixed name, so golden files do not change.
func withStableURL(czar *fakeCzar, text string) string {
	return strings.ReplaceAll(text, czar.server.URL, "https://czar.test")
}

func ok(fixture string) []reply { return []reply{{http.StatusOK, fixture}} }

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "fixtures", name+".json"))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// result is one run of the CLI.
type result struct {
	stdout, stderr string
	code           int
	sleeps         []time.Duration
	configPath     string
}

// options change one run. Zero values mean: signed in by PULSECZAR_TOKEN, piped output.
type options struct {
	terminal bool
	noToken  bool
	env      map[string]string
}

func runCLI(t *testing.T, czar *fakeCzar, opts options, args ...string) result {
	t.Helper()

	var out, errOut bytes.Buffer
	run := result{configPath: filepath.Join(t.TempDir(), "pulseczar", "config.json")}

	env := map[string]string{envURL: czar.server.URL}
	if !opts.noToken {
		env[envToken] = testToken
	}
	for key, value := range opts.env {
		env[key] = value
	}

	run.code = Run(args, Env{
		Stdout:     &out,
		Stderr:     &errOut,
		Getenv:     func(key string) string { return env[key] },
		ConfigPath: run.configPath,
		IsTerminal: opts.terminal,
		HTTP:       czar.server.Client(),
		Sleep:      func(d time.Duration) { run.sleeps = append(run.sleeps, d) },
		Hostname:   "ada-laptop",
	})
	run.stdout, run.stderr = withStableURL(czar, out.String()), withStableURL(czar, errOut.String())

	return run
}

// assertGolden compares got with testdata/golden/<name>.txt. Run `go test ./... -update`
// to rewrite the files after a change you meant to make.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", "golden", name+".txt")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if string(want) != got {
		t.Errorf("%s differs\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}
