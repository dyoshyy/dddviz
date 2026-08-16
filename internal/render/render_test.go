package render_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dyoshyy/dddviz/internal/model"
	"github.com/dyoshyy/dddviz/internal/render"
)

// 出力は 1.6MB あるためゴールデン比較はしない。
// 自己完結であることと、データが埋まっていることだけ確かめる。
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
		"Order", // 解析結果が埋まっている
		"OrderID",
		"PricingService",
		"ELK",    // elkjs が同梱されている
		"#stage", // スタイルが同梱されている
	} {
		if !strings.Contains(out, want) {
			t.Errorf("出力に %q が含まれない", want)
		}
	}

	// 外部への参照が残っていれば自己完結が崩れている。
	for _, bad := range []string{"src=\"http", "href=\"http", "//unpkg.com", "//cdn."} {
		if strings.Contains(out, bad) {
			t.Errorf("外部参照が残っている: %q", bad)
		}
	}

	if buf.Len() < 500_000 {
		t.Errorf("elkjs が埋め込まれていない可能性がある（%d バイト）", buf.Len())
	}
}
