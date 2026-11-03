package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/pulseczar-com/cli/internal/render"
)

// The status line is for status bars: one line, always exit 0, so a bar can show
// "offline" or "signed out" instead of dropping the entry on a failed exit.
const (
	lineName = "PulseCzar"
	// A cache younger than lineFreshFor is printed with no API call.
	lineFreshFor = 30 * time.Second
	// When the call fails, a cache younger than lineStaleFor is printed with its age.
	lineStaleFor = time.Hour
	lineTimeout  = 2 * time.Second

	cacheFilePrefix = "status-"
	cacheFileSuffix = ".json"
)

// statusLine builds the line for the bar. It never fails: every problem is a word on the line.
func (a *app) statusLine() string {
	found, err := a.loadSettings()
	if err != nil {
		return a.noticeLine("?", "offline")
	}
	if found.Token == "" {
		return a.noticeLine("○", "signed out")
	}

	cache := a.statusCacheFor(found)
	cached, haveCache := cache.read()
	age := a.env.Now().Sub(cached.FetchedAt)

	if haveCache && isWithin(age, lineFreshFor) {
		return a.dataLine(cached.Status, "")
	}

	status, err := a.fetchStatusQuickly(found)
	switch {
	case err == nil:
		cache.write(statusCacheEntry{FetchedAt: a.env.Now(), Status: status})

		return a.dataLine(status, "")
	case isRefusedToken(err):
		cache.remove()

		return a.noticeLine("○", "sign in again")
	case haveCache && isWithin(age, lineStaleFor):
		return a.dataLine(cached.Status, shortAge(age))
	default:
		return a.noticeLine("?", "offline")
	}
}

// isWithin is false for a cache dated in the future, so a clock set back never counts as fresh.
func isWithin(age, limit time.Duration) bool {
	return age >= 0 && age < limit
}

func (a *app) fetchStatusQuickly(found settings) (api.Status, error) {
	quick := *a.env.HTTP
	quick.Timeout = lineTimeout

	client := &api.Client{BaseURL: found.URL, Token: found.Token, HTTP: &quick}
	status, _, err := client.GetStatus()

	return status, err
}

// isRefusedToken is true when the server answered 401 or 403 to the token.
func isRefusedToken(err error) bool {
	var apiErr *api.Error

	return errors.As(err, &apiErr) && (apiErr.Code == api.CodeUnauthenticated || apiErr.Code == api.CodeForbidden)
}

// dataLine shows the server's own status text, like the web header. A non-empty age
// is the dim "6m ago" of a cached line.
func (a *app) dataLine(status api.Status, age string) string {
	style := a.styleFor(true)
	mark, color := lineMark(status.Counts)
	line := lineName + " " + style.Paint(color, mark) + " " + status.Summary.Text

	if age != "" {
		line += style.Paint(render.Gray, " · "+age+" ago")
	}

	return line
}

func (a *app) noticeLine(mark, text string) string {
	return lineName + " " + a.styleFor(true).Paint(render.Gray, mark) + " " + text
}

// lineMark picks the dot from the counts. Green is for an account with something up
// and nothing down or stale; new monitors alone are never green.
func lineMark(counts api.Counts) (string, render.Color) {
	switch {
	case counts.Down > 0:
		return "●", render.Red
	case counts.Stale > 0:
		return "●", render.Yellow
	case counts.Total == 0:
		return "◌", render.Gray
	case counts.Paused == counts.Total:
		return "○", render.Gray
	case counts.Up > 0:
		return "●", render.Green
	default:
		return "◌", render.Gray
	}
}

// shortAge writes how old a cached line is: "40s", "6m", "2h".
func shortAge(age time.Duration) string {
	switch {
	case age < time.Minute:
		return fmt.Sprintf("%ds", int(age/time.Second))
	case age < time.Hour:
		return fmt.Sprintf("%dm", int(age/time.Minute))
	default:
		return fmt.Sprintf("%dh", int(age/time.Hour))
	}
}

// statusCacheEntry is the file: the answer and when it arrived.
type statusCacheEntry struct {
	FetchedAt time.Time  `json:"fetched_at"`
	Status    api.Status `json:"status"`
}

// statusCache is one file in the user cache folder. An empty path is no cache.
type statusCache struct {
	path string
}

// statusCacheFor names the file by a hash of the server and the token, so two
// accounts never share it and the token itself is never written.
func (a *app) statusCacheFor(found settings) statusCache {
	if a.env.CacheDir == "" {
		return statusCache{}
	}

	sum := sha256.Sum256([]byte(found.URL + "\n" + found.Token))
	name := cacheFilePrefix + hex.EncodeToString(sum[:8]) + cacheFileSuffix

	return statusCache{path: filepath.Join(a.env.CacheDir, name)}
}

func (c statusCache) read() (statusCacheEntry, bool) {
	var entry statusCacheEntry

	data, err := os.ReadFile(c.path)
	if c.path == "" || err != nil {
		return entry, false
	}

	return entry, json.Unmarshal(data, &entry) == nil
}

// write is best effort: a bar that cannot cache still shows the line.
func (c statusCache) write(entry statusCacheEntry) {
	if c.path == "" {
		return
	}

	data, err := json.Marshal(entry)
	if err != nil || os.MkdirAll(filepath.Dir(c.path), 0o700) != nil {
		return
	}

	_ = os.WriteFile(c.path, data, 0o600)
}

func (c statusCache) remove() {
	if c.path != "" {
		_ = os.Remove(c.path)
	}
}

// clearStatusCaches deletes the cache of every account, for `auth login` and `auth logout`.
func (a *app) clearStatusCaches() {
	if a.env.CacheDir == "" {
		return
	}

	files, _ := filepath.Glob(filepath.Join(a.env.CacheDir, cacheFilePrefix+"*"+cacheFileSuffix))
	for _, file := range files {
		_ = os.Remove(file)
	}
}
