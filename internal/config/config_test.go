package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOfAMissingFileIsEmpty(t *testing.T) {
	file, err := Load(filepath.Join(t.TempDir(), "none.json"))

	if err != nil || file != (File{}) {
		t.Errorf("got %+v, %v", file, err)
	}
}

func TestSaveThenLoadRoundTripsAndKeepsModeAt0600(t *testing.T) {
	path := DefaultPath(t.TempDir())
	want := File{URL: "https://pulseczar.com", Token: "1|abc"}

	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil || got != want {
		t.Errorf("got %+v, %v", got, err)
	}
	assertMode(t, path, 0o600)
}

func TestSaveTightensAFileThatWasWorldReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Save(path, File{Token: "1|abc"}); err != nil {
		t.Fatal(err)
	}

	assertMode(t, path, 0o600)
}

func TestLoadRejectsAFileThatIsNotJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Error("want an error")
	}
}

func TestRemoveOfAMissingFileIsNotAnError(t *testing.T) {
	if err := Remove(filepath.Join(t.TempDir(), "none.json")); err != nil {
		t.Error(err)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != want {
		t.Errorf("mode = %o, want %o", info.Mode().Perm(), want)
	}
}
