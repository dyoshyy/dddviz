// Package render は model.Graph を単一の HTML に書き出す。
//
// JS と CSS と elkjs をバイナリに埋め込み、解析結果の JSON ごと
// 1 枚に詰める。出力を開くのに何も要らない状態を保つ。
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

//go:embed all:assets
var assets embed.FS

// HTML は graph を描画する自己完結の HTML を w に書く。
func HTML(w io.Writer, graph *model.Graph) error {
	tmplSrc, err := assets.ReadFile("assets/template.html")
	if err != nil {
		return fmt.Errorf("テンプレートの読み込み: %w", err)
	}
	css, err := assets.ReadFile("assets/style.css")
	if err != nil {
		return fmt.Errorf("CSS の読み込み: %w", err)
	}
	js, err := assets.ReadFile("assets/app.js")
	if err != nil {
		return fmt.Errorf("JS の読み込み: %w", err)
	}
	elk, err := assets.ReadFile("assets/elk.bundled.js")
	if err != nil {
		return fmt.Errorf("elkjs の読み込み: %w", err)
	}

	// MarshalIndent は既定で <, >, & を < 等に置き換えるので、
	// script 要素の中に置いてもタグとして解釈されない。
	data, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("JSON への変換: %w", err)
	}

	tmpl, err := template.New("page").Parse(string(tmplSrc))
	if err != nil {
		return fmt.Errorf("テンプレートの解析: %w", err)
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
		"ELK":   string(elk),
		"Data":  string(data),
	})
	if err != nil {
		return fmt.Errorf("テンプレートの展開: %w", err)
	}

	_, err = w.Write(buf.Bytes())
	return err
}
