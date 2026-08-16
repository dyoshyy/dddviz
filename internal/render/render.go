// Package render writes a model.Graph out as a single HTML file.
//
// The bundled JS and the CSS are embedded in the binary and packed into one
// page along with the analysis JSON, so that opening the output requires
// nothing else.
//
// assets/app.js is a build artifact of web/. It is committed so that using
// dddviz needs only Go; Node is required to change the rendering code, not
// to run it. Rebuild with `go generate ./internal/render`.
package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/dyoshyy/dddviz/internal/model"
)

//go:generate sh -c "cd ../../web && npm ci --silent && npm run build"

//go:embed all:assets
var assets embed.FS

// Options controls how the page behaves once it is open.
type Options struct {
	// Live makes the page subscribe to /events and redraw whenever the
	// server pushes a new graph. Used by -watch.
	Live bool
}

// HTML writes a self-contained page rendering graph to w.
func HTML(w io.Writer, graph *model.Graph) error {
	return Page(w, graph, Options{})
}

// Page writes the rendered graph to w with the given options.
func Page(w io.Writer, graph *model.Graph, opt Options) error {
	tmplSrc, err := assets.ReadFile("assets/template.html")
	if err != nil {
		return fmt.Errorf("reading template: %w", err)
	}
	css, err := assets.ReadFile("assets/style.css")
	if err != nil {
		return fmt.Errorf("reading CSS: %w", err)
	}
	js, err := assets.ReadFile("assets/app.js")
	if err != nil {
		return fmt.Errorf("reading JS: %w", err)
	}
	// encoding/json escapes <, > and & by default, so the payload is safe
	// to place inside a script element.
	data, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	tmpl, err := template.New("page").Parse(string(tmplSrc))
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	title := graph.Meta.Title
	if title == "" {
		title = "dddviz"
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{
		"Title": title,
		"CSS":   string(css),
		"JS":    string(js),
		"Data":  string(data),
		"Live":  opt.Live,
	})
	if err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	_, err = w.Write(buf.Bytes())
	return err
}
