package analyze_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/dyoshyy/dddviz/internal/analyze"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func TestAnalyze(t *testing.T) {
	cases := []string{"basic"}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join("testdata", name)

			got, err := analyze.Load(dir, "./...")
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			enc, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			enc = append(enc, '\n')

			golden := filepath.Join(dir, "golden.json")
			if *update {
				if err := os.WriteFile(golden, enc, 0o644); err != nil {
					t.Fatalf("writing golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("reading golden (run with -update to create it): %v", err)
			}

			if string(enc) != string(want) {
				t.Errorf("analysis does not match the golden file\n--- got\n%s\n--- want\n%s", enc, want)
			}
		})
	}
}
