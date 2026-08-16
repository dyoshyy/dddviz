// Command dddviz は Go の DDD コードから集約の地図を描く。
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
	"github.com/dyoshyy/dddviz/internal/render"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "dddviz:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dir    = flag.String("C", ".", "解析の起点ディレクトリ")
		out    = flag.String("o", "", "出力先ファイル（省略時は標準出力）")
		format = flag.String("format", "", "出力形式 html または json（省略時は -o の拡張子から決める）")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "使い方: dddviz [オプション] [パッケージパターン...]")
		fmt.Fprintln(os.Stderr, "\n例:")
		fmt.Fprintln(os.Stderr, "  dddviz -C ~/repos/myapp -o docs/domain.html ./internal/...")
		fmt.Fprintln(os.Stderr, "  dddviz -format json ./internal/...")
		fmt.Fprintln(os.Stderr, "\nオプション:")
		flag.PrintDefaults()
	}
	flag.Parse()

	f, err := resolveFormat(*format, *out)
	if err != nil {
		return err
	}

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
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
			return fmt.Errorf("JSON への変換: %w", err)
		}
	}

	if *out != "" {
		fmt.Fprintf(os.Stderr, "%s に書き出した（%d 集約 / %d 参照 / 未分類 %d）\n",
			*out, len(graph.Aggregates), len(graph.References), len(graph.Unclassified))
	}
	return nil
}

// resolveFormat は出力形式を決める。明示指定が無ければ出力先の拡張子から、
// それも無ければ標準出力向けに json を選ぶ。
func resolveFormat(format, out string) (string, error) {
	if format != "" {
		if format != "html" && format != "json" {
			return "", fmt.Errorf("未対応の形式: %s（html か json）", format)
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
		return "", fmt.Errorf("拡張子から形式を判断できない: %s（-format で指定）", out)
	}
}

func openOutput(path string) (io.Writer, func(), error) {
	if path == "" {
		bw := bufio.NewWriter(os.Stdout)
		return bw, func() { bw.Flush() }, nil
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("出力先ディレクトリの作成: %w", err)
		}
	}
	fh, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("出力先を開けない: %w", err)
	}
	bw := bufio.NewWriter(fh)
	return bw, func() { bw.Flush(); fh.Close() }, nil
}
