// Command dddviz は Go の DDD コードから集約の構造を抽出する。
//
// POC 1 の段階では解析結果を JSON で出すところまで。図の描画は POC 2。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/dyoshyy/dddviz/internal/analyze"
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
		format = flag.String("format", "json", "出力形式（現在は json のみ）")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "使い方: dddviz [オプション] [パッケージパターン...]")
		fmt.Fprintln(os.Stderr, "\n例: dddviz -C ~/repos/myapp ./internal/...")
		fmt.Fprintln(os.Stderr, "\nオプション:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *format != "json" {
		return fmt.Errorf("未対応の形式: %s", *format)
	}

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	graph, err := analyze.Load(*dir, patterns...)
	if err != nil {
		return err
	}

	enc, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON への変換: %w", err)
	}
	enc = append(enc, '\n')

	if *out == "" {
		_, err = os.Stdout.Write(enc)
		return err
	}
	return os.WriteFile(*out, enc, 0o644)
}
