package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprintTracksGoFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.go"), "package a\n")

	first, err := fingerprint(dir)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	if first == "" {
		t.Fatal("fingerprint is empty")
	}

	if again, _ := fingerprint(dir); again != first {
		t.Error("fingerprint changed while nothing did")
	}

	// Editing a file must move the fingerprint. Size changes here, which
	// also keeps the test off the filesystem's mtime resolution.
	write(t, filepath.Join(dir, "a.go"), "package a\n\nvar X = 1\n")
	edited, _ := fingerprint(dir)
	if edited == first {
		t.Error("fingerprint did not change after an edit")
	}

	// A new file counts too.
	write(t, filepath.Join(dir, "b.go"), "package a\n")
	added, _ := fingerprint(dir)
	if added == edited {
		t.Error("fingerprint did not change after adding a file")
	}
}

func TestFingerprintIgnoresNonGoAndSkippedDirs(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.go"), "package a\n")
	base, _ := fingerprint(dir)

	write(t, filepath.Join(dir, "README.md"), "hello")
	if got, _ := fingerprint(dir); got != base {
		t.Error("a non-Go file moved the fingerprint")
	}

	for _, skipped := range []string{".git", "node_modules", "vendor"} {
		write(t, filepath.Join(dir, skipped, "x.go"), "package x\n")
		if got, _ := fingerprint(dir); got != base {
			t.Errorf("a .go file under %s moved the fingerprint", skipped)
		}
	}
}

func TestFingerprintIncludesTestdata(t *testing.T) {
	// testdata holds the fixtures a diagram is often built from, so edits
	// there must trigger a redraw.
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.go"), "package a\n")
	base, _ := fingerprint(dir)

	write(t, filepath.Join(dir, "testdata", "fixture.go"), "package fixture\n")
	if got, _ := fingerprint(dir); got == base {
		t.Error("a .go file under testdata did not move the fingerprint")
	}
}

func TestBroadcastSkipsBlockedClients(t *testing.T) {
	s := &server{}

	live := make(chan []byte, 1)
	stuck := make(chan []byte) // unbuffered and never read
	s.addClient(live)
	s.addClient(stuck)

	done := make(chan struct{})
	go func() {
		s.broadcast("graph", []byte(`{"ok":true}`))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on a client that was not reading")
	}

	select {
	case msg := <-live:
		want := "event: graph\ndata: {\"ok\":true}\n\n"
		if string(msg) != want {
			t.Errorf("got %q, want %q", msg, want)
		}
	default:
		t.Error("the reading client got nothing")
	}
}
