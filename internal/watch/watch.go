// Package watch serves the diagram and redraws it as the code changes.
//
// It deliberately uses nothing outside the standard library: the appeal of
// this tool is that `go install` is the whole setup, and a file watcher or a
// WebSocket library would spend that for very little.
//
// Changes are pushed over server-sent events as a fresh graph rather than as
// a reload. A reload would refetch the ~1.6MB page every time and, more to
// the point, would throw away which aggregates the reader had expanded.
package watch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dyoshyy/dddviz/internal/analyze"
	"github.com/dyoshyy/dddviz/internal/model"
	"github.com/dyoshyy/dddviz/internal/render"
)

// Options configures a watch session.
type Options struct {
	Dir      string
	Patterns []string
	// Port of 0 asks the OS for a free one.
	Port int
	// OpenPage launches a browser once the server is listening.
	OpenPage bool
	// Interval between scans of the tree. Zero means the default.
	Interval time.Duration
	Log      io.Writer
}

const defaultInterval = 500 * time.Millisecond

// skipDirs are never scanned for changes.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"testdata":     false, // testdata is worth watching; listed for clarity
}

// Serve analyzes once, starts the server, and keeps the page in step with
// the code until the process is interrupted.
func Serve(opt Options) error {
	if opt.Interval == 0 {
		opt.Interval = defaultInterval
	}
	if opt.Log == nil {
		opt.Log = io.Discard
	}

	// Fail before opening a browser if the code cannot be analyzed at all.
	graph, err := analyze.Load(opt.Dir, opt.Patterns...)
	if err != nil {
		return err
	}

	s := &server{opt: opt, graph: graph}

	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(opt.Port))
	if err != nil {
		return fmt.Errorf("listening: %w", err)
	}
	url := "http://" + ln.Addr().String()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handlePage)
	mux.HandleFunc("/events", s.handleEvents)

	go s.poll()

	fmt.Fprintf(opt.Log, "dddviz: serving %s (%d aggregates, %d references)\n",
		url, len(graph.Aggregates), len(graph.References))
	fmt.Fprintf(opt.Log, "dddviz: watching %s -- press Ctrl-C to stop\n", opt.Dir)

	if opt.OpenPage {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(opt.Log, "dddviz: could not open a browser (%v); visit %s\n", err, url)
		}
	}

	return http.Serve(ln, mux)
}

type server struct {
	opt Options

	mu      sync.Mutex
	graph   *model.Graph
	clients map[chan []byte]bool
}

func (s *server) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	graph := s.graph
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The page carries its own state, so a stale cached copy would be
	// confusing rather than fast.
	w.Header().Set("Cache-Control", "no-store")
	if err := render.Page(w, graph, render.Options{Live: true}); err != nil {
		fmt.Fprintf(s.opt.Log, "dddviz: rendering the page: %v\n", err)
	}
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 4)
	s.addClient(ch)
	defer s.removeClient(ch)

	flusher.Flush()

	// A periodic comment keeps proxies and idle timeouts from closing the
	// stream on a codebase nobody is editing right now.
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			if _, err := w.Write(msg); err != nil {
				return
			}
			flusher.Flush()
		case <-ping.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *server) addClient(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clients == nil {
		s.clients = map[chan []byte]bool{}
	}
	s.clients[ch] = true
}

func (s *server) removeClient(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, ch)
}

// broadcast sends one SSE message to every connected page. A page that
// cannot keep up is skipped rather than allowed to block the scan loop.
func (s *server) broadcast(event string, payload []byte) {
	var b strings.Builder
	b.WriteString("event: ")
	b.WriteString(event)
	b.WriteString("\ndata: ")
	b.Write(payload)
	b.WriteString("\n\n")
	msg := []byte(b.String())

	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

// poll rescans the tree and re-analyzes whenever the fingerprint moves.
func (s *server) poll() {
	last, _ := fingerprint(s.opt.Dir)

	for range time.Tick(s.opt.Interval) {
		fp, err := fingerprint(s.opt.Dir)
		if err != nil || fp == last {
			continue
		}
		last = fp

		graph, err := analyze.Load(s.opt.Dir, s.opt.Patterns...)
		if err != nil {
			// Code in the middle of an edit does not compile, which is
			// normal here. Show it on the page and wait for the next save.
			msg, _ := json.Marshal(err.Error())
			s.broadcast("failed", msg)
			continue
		}

		payload, err := json.Marshal(graph)
		if err != nil {
			continue
		}

		s.mu.Lock()
		s.graph = graph
		s.mu.Unlock()

		s.broadcast("graph", payload)
		fmt.Fprintf(s.opt.Log, "dddviz: redrew (%d aggregates, %d references, %d unclassified)\n",
			len(graph.Aggregates), len(graph.References), len(graph.Unclassified))
	}
}

// fingerprint summarizes the .go files under dir by size and mtime.
func fingerprint(dir string) (string, error) {
	var entries []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable corner should not stop the scan
		}
		if d.IsDir() {
			name := d.Name()
			if skipDirs[name] || (strings.HasPrefix(name, ".") && name != "." && path != dir) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, fmt.Sprintf("%s\x00%d\x00%d",
			path, info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(entries)
	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// openBrowser asks the desktop to open url.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	return exec.Command(cmd, append(args, url)...).Start()
}
