// Command dddviz draws a map of the aggregates in a Go DDD codebase.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dyoshyy/dddviz/internal/analyze"
	"github.com/dyoshyy/dddviz/internal/model"
	"github.com/dyoshyy/dddviz/internal/render"
	"github.com/dyoshyy/dddviz/internal/watch"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "dddviz:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dir     = flag.String("C", ".", "directory to analyze from")
		out     = flag.String("o", "", "output file (defaults to stdout)")
		format  = flag.String("format", "", "output format, html or json (inferred from -o when unset)")
		watchOn = flag.Bool("watch", false, "serve the diagram and redraw it as the code changes")
		port    = flag.Int("port", 0, "port for -watch (0 picks a free one)")
		noOpen  = flag.Bool("no-open", false, "with -watch, do not open a browser")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: dddviz [flags] [packages...]")
		fmt.Fprintln(os.Stderr, "\nexamples:")
		fmt.Fprintln(os.Stderr, "  dddviz -C ~/repos/myapp -o docs/domain.html ./internal/...")
		fmt.Fprintln(os.Stderr, "  dddviz -watch -C ~/repos/myapp ./internal/...")
		fmt.Fprintln(os.Stderr, "  dddviz -format json ./internal/...")
		fmt.Fprintln(os.Stderr, "\nflags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	if *watchOn {
		return watch.Serve(watch.Options{
			Dir:      *dir,
			Patterns: patterns,
			Port:     *port,
			OpenPage: !*noOpen,
			Log:      os.Stderr,
		})
	}

	f, err := resolveFormat(*format, *out)
	if err != nil {
		return err
	}

	graph, err := analyze.Load(*dir, patterns...)
	if err != nil {
		return err
	}

	w, closeOut, err := openOutput(*out)
	if err != nil {
		return err
	}
	defer closeOut()

	switch f {
	case "html":
		if err := render.HTML(w, graph); err != nil {
			return err
		}
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(graph); err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
	}

	if *out != "" {
		fmt.Fprintf(os.Stderr, "wrote %s (%d aggregates, %d references, %d unclassified)\n",
			*out, len(graph.Aggregates), len(graph.References), len(graph.Unclassified))
	}
	if len(graph.Aggregates) == 0 {
		reportNoAggregates(os.Stderr, graph)
	} else if len(graph.Meta.Packages) > len(graph.Meta.DomainPackages) {
		// Types outside the packages holding aggregates are application or
		// infrastructure concerns, not part of the model. Say so rather than
		// leaving someone to wonder where their use cases went.
		fmt.Fprintf(os.Stderr,
			"dddviz: aggregates live in %d of %d analyzed packages; types elsewhere are not part of the model\n",
			len(graph.Meta.DomainPackages), len(graph.Meta.Packages))
	}
	return nil
}

// reportNoAggregates explains an empty diagram. Nothing marked is by far the
// most likely first run, and an empty page on its own looks like a failure.
func reportNoAggregates(w io.Writer, graph *model.Graph) {
	fmt.Fprintln(w, "\ndddviz: no aggregates found -- nothing is marked with //ddd:aggregate")

	if len(graph.Candidates) > 0 {
		fmt.Fprintln(w, "\n  Candidates, from types that own an ID type:")
		for i, c := range graph.Candidates {
			if i == maxCandidates {
				fmt.Fprintf(w, "    ... and %d more\n", len(graph.Candidates)-maxCandidates)
				break
			}
			fmt.Fprintf(w, "    %-24s %-22s (%s)\n", c.Name, c.Pos, c.IDType)
		}
		fmt.Fprintln(w, "\n  Owning an ID does not make a type an aggregate root, so treat")
		fmt.Fprintln(w, "  these as a starting point rather than an answer.")
	}

	fmt.Fprintln(w, "\n  Mark an aggregate root and run again:")
	fmt.Fprintln(w, "\n    //ddd:aggregate")
	fmt.Fprintln(w, "    type Order struct {")
	fmt.Fprintln(w, "")
}

const maxCandidates = 12

// resolveFormat picks the output format: an explicit flag first, then the
// output file's extension, then JSON for stdout.
func resolveFormat(format, out string) (string, error) {
	if format != "" {
		if format != "html" && format != "json" {
			return "", fmt.Errorf("unsupported format %q (want html or json)", format)
		}
		return format, nil
	}
	switch strings.ToLower(filepath.Ext(out)) {
	case ".html", ".htm":
		return "html", nil
	case ".json":
		return "json", nil
	case "":
		return "json", nil
	default:
		return "", fmt.Errorf("cannot infer a format from %q (use -format)", out)
	}
}

func openOutput(path string) (io.Writer, func(), error) {
	if path == "" {
		bw := bufio.NewWriter(os.Stdout)
		return bw, func() { bw.Flush() }, nil
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("creating output directory: %w", err)
		}
	}
	fh, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening output file: %w", err)
	}
	bw := bufio.NewWriter(fh)
	return bw, func() { bw.Flush(); fh.Close() }, nil
}
