package render_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dyoshyy/dddviz/internal/model"
	"github.com/dyoshyy/dddviz/internal/render"
)

// The output is about 1.6MB, so there is no golden comparison here.
// This only checks that the page is self-contained and carries the data.
func TestHTML(t *testing.T) {
	g := &model.Graph{
		Meta: model.Meta{Title: "training", Packages: []string{"example.com/x/training"}},
		Aggregates: []model.Aggregate{{
			Name: "Order", Pkg: "example.com/x/training", Pos: "order.go:5",
			IDType: "OrderID",
			Fields: []model.Field{{Name: "id", Type: "OrderID"}},
			Members: []model.Member{{
				Name: "Money", Kind: model.KindVO, Depth: 1,
				Fields: []model.Field{{Name: "amount", Type: "int64"}},
			}},
		}},
		References:   []model.Reference{{From: "Order", To: "Customer", Via: "Order.customer CustomerID"}},
		Unclassified: []model.Unclassified{{Name: "PricingService", Pos: "pricing.go:3"}},
	}

	var buf bytes.Buffer
	if err := render.HTML(&buf, g); err != nil {
		t.Fatalf("HTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"<title>training</title>",
		"window.__DDDVIZ__",
		"Order", // the analysis result is embedded
		"OrderID",
		"PricingService",
		"dagre",  // the layout engine is bundled into the page
		"#stage", // the stylesheet ships with the page
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q", want)
		}
	}

	// Any remaining external reference would break self-containment.
	for _, bad := range []string{"src=\"http", "href=\"http", "//unpkg.com", "//cdn."} {
		if strings.Contains(out, bad) {
			t.Errorf("external reference left in the output: %q", bad)
		}
	}

	// The bundle carries the layout engine and the rendering code. A page
	// far below this size means one of them failed to embed.
	if buf.Len() < 40_000 {
		t.Errorf("the JS bundle may not be embedded (%d bytes)", buf.Len())
	}
	// And a page far above it means something heavy crept back in.
	if buf.Len() > 300_000 {
		t.Errorf("the page has grown unexpectedly large (%d bytes)", buf.Len())
	}
}
