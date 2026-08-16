// Package analyze は Go のソースを読み、集約の構造を model.Graph に組み立てる。
//
// 設計の中核は「推論できるものは全部推論し、原理的に推論不能な1点だけ
// 人間に聞く」こと。人間が書くのは //ddd:aggregate の1行だけで、
// 集約の中身・Entity/VO の別・ID 型の対応づけ・集約間の参照は
// すべてコードから導く。
package analyze

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/dyoshyy/dddviz/internal/model"
)

const (
	markerAggregate = "ddd:aggregate"
	markerID        = "ddd:id"
)

// Load は dir を起点に patterns のパッケージを解析する。
func Load(dir string, patterns ...string) (*model.Graph, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir: dir,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("パッケージのロード: %w", err)
	}
	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, e.Error())
		}
	})
	if len(loadErrs) > 0 {
		return nil, fmt.Errorf("解析対象にエラーがある:\n%s", strings.Join(loadErrs, "\n"))
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("パッケージが見つからない: %v", patterns)
	}

	a := &analyzer{
		fset:      pkgs[0].Fset,
		types:     map[string]*declared{},
		inScope:   map[string]bool{},
		aggById:   map[string]string{},
		identity:  map[string]bool{},
		aggregate: map[string]*declared{},
	}
	for _, p := range pkgs {
		a.collect(p)
	}
	a.resolveIDTypes()
	a.resolveIdentityTypes()
	return a.build(), nil
}

// declared は解析対象パッケージで宣言された名前付き型ひとつ。
type declared struct {
	named   *types.Named
	pkgPath string
	pkgName string
	name    string
	pos     token.Position
	// isAggregate は //ddd:aggregate が付いているか。
	isAggregate bool
	// idFor は //ddd:id for=X の X。無ければ空。
	idFor string
}

func (d *declared) key() string { return d.pkgPath + "." + d.name }

type analyzer struct {
	fset *token.FileSet
	// types は解析対象で宣言された全ての名前付き型。キーは pkgPath.Name。
	types map[string]*declared
	// inScope は解析対象パッケージのパス。
	inScope map[string]bool
	// aggById は集約 ID 型のキーから、それが指す集約名への対応。
	aggById map[string]string
	// identity は識別子型のキー。集約 ID 型に加え、Entity の ID 型も含む。
	// 箱として描く意味がないので、中身にも未分類にも出さない。
	identity map[string]bool
	// aggregate は集約ルートのキーから宣言へ。
	aggregate map[string]*declared
}

func (a *analyzer) collect(p *packages.Package) {
	a.inScope[p.PkgPath] = true

	markers := map[string][]string{} // 型名 → マーカー行
	for _, syn := range p.Syntax {
		for _, decl := range syn.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				doc := ts.Doc
				// 単独宣言では doc が GenDecl 側に付く。括弧でまとめた
				// 宣言の doc を個々の型に誤って配らないよう、1件のときだけ拾う。
				if doc == nil && len(gd.Specs) == 1 {
					doc = gd.Doc
				}
				if ms := parseMarkers(doc); len(ms) > 0 {
					markers[ts.Name.Name] = append(markers[ts.Name.Name], ms...)
				}
			}
		}
	}

	scope := p.Types.Scope()
	for _, name := range scope.Names() {
		obj, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || obj.IsAlias() {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		d := &declared{
			named:   named,
			pkgPath: p.PkgPath,
			pkgName: p.Name,
			name:    name,
			pos:     a.fset.Position(obj.Pos()),
		}
		for _, m := range markers[name] {
			switch {
			case m == markerAggregate:
				d.isAggregate = true
			case strings.HasPrefix(m, markerID):
				d.idFor = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(m, markerID)), "for=")
			}
		}
		a.types[d.key()] = d
		if d.isAggregate {
			a.aggregate[d.key()] = d
		}
	}
}

// parseMarkers は doc コメントから //ddd: で始まる行を取り出す。
func parseMarkers(doc *ast.CommentGroup) []string {
	if doc == nil {
		return nil
	}
	var out []string
	for _, c := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if strings.HasPrefix(text, "ddd:") {
			out = append(out, text)
		}
	}
	return out
}

// resolveIDTypes は ID 型と集約の対応づけを決める。
//
// 既定では集約ルート X に対し XID という名前の型を対応させる。
// 命名が規約から外れる場合だけ //ddd:id for=X で明示する。
func (a *analyzer) resolveIDTypes() {
	for _, agg := range a.aggregate {
		candidate := agg.pkgPath + "." + agg.name + "ID"
		if _, ok := a.types[candidate]; ok {
			a.aggById[candidate] = agg.name
		}
	}
	// 明示指定は暗黙の対応づけより優先する。
	for key, d := range a.types {
		if d.idFor == "" {
			continue
		}
		for _, agg := range a.aggregate {
			if agg.name == d.idFor {
				a.aggById[key] = agg.name
			}
		}
	}
}

// resolveIdentityTypes は識別子型を洗い出す。
//
// XID という名前の型で X が解析対象に存在するものを識別子とみなす。
// 集約 ID 型（Order → OrderID）だけでなく、集約の中の Entity の ID 型
// （Shipment → ShipmentID）も含む。どちらも箱として描く価値がない。
func (a *analyzer) resolveIdentityTypes() {
	for key := range a.aggById {
		a.identity[key] = true
	}
	for key, d := range a.types {
		owner := strings.TrimSuffix(d.name, "ID")
		if owner == d.name || owner == "" {
			continue
		}
		if _, ok := a.types[d.pkgPath+"."+owner]; ok {
			a.identity[key] = true
		}
	}
}

func (a *analyzer) build() *model.Graph {
	g := &model.Graph{
		Meta:         a.meta(),
		Aggregates:   []model.Aggregate{},
		References:   []model.Reference{},
		Unclassified: []model.Unclassified{},
	}

	// どの集約からか到達できた型。未分類の判定に使う。
	reached := map[string]bool{}

	for _, agg := range a.aggregate {
		built, refs := a.buildAggregate(agg, reached)
		g.Aggregates = append(g.Aggregates, built)
		g.References = append(g.References, refs...)
	}

	for key, d := range a.types {
		if d.isAggregate || reached[key] {
			continue
		}
		if a.identity[key] {
			continue
		}
		g.Unclassified = append(g.Unclassified, model.Unclassified{
			Name: d.name,
			Pkg:  d.pkgPath,
			Pos:  shortPos(d.pos),
		})
	}

	sort.Slice(g.Aggregates, func(i, j int) bool { return g.Aggregates[i].Name < g.Aggregates[j].Name })
	sort.Slice(g.References, func(i, j int) bool {
		x, y := g.References[i], g.References[j]
		if x.From != y.From {
			return x.From < y.From
		}
		if x.To != y.To {
			return x.To < y.To
		}
		return x.Via < y.Via
	})
	sort.Slice(g.Unclassified, func(i, j int) bool { return g.Unclassified[i].Name < g.Unclassified[j].Name })
	return g
}

// meta は図の見出しに使う情報を組み立てる。
// 表題は解析対象パッケージの共通接頭辞から取る。
func (a *analyzer) meta() model.Meta {
	pkgs := make([]string, 0, len(a.inScope))
	for p := range a.inScope {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	title := "domain"
	if len(pkgs) == 1 {
		title = pkgs[0]
	} else if len(pkgs) > 1 {
		title = commonPrefix(pkgs)
	}
	if i := strings.LastIndex(title, "/"); i >= 0 && i < len(title)-1 {
		title = title[i+1:]
	}
	return model.Meta{Title: title, Packages: pkgs}
}

// commonPrefix はパッケージパスの共通部分をスラッシュ区切りで返す。
func commonPrefix(paths []string) string {
	parts := strings.Split(paths[0], "/")
	for _, p := range paths[1:] {
		cur := strings.Split(p, "/")
		n := 0
		for n < len(parts) && n < len(cur) && parts[n] == cur[n] {
			n++
		}
		parts = parts[:n]
	}
	return strings.Join(parts, "/")
}

// buildAggregate は集約ルートから到達可能な型を幅優先でたどる。
func (a *analyzer) buildAggregate(root *declared, reached map[string]bool) (model.Aggregate, []model.Reference) {
	out := model.Aggregate{
		Name:    root.name,
		Pkg:     root.pkgPath,
		Pos:     shortPos(root.pos),
		Members: []model.Member{},
		Fields:  a.fieldsOf(root),
	}
	if key, ok := a.idTypeOf(root); ok {
		out.IDType = a.types[key].name
	}

	var refs []model.Reference
	// seen は同一集約内での重複展開を防ぐ。集約をまたぐ重複は許す。
	seen := map[string]bool{root.key(): true}
	type item struct {
		d     *declared
		depth int
	}
	queue := []item{{d: root, depth: 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, f := range structFields(cur.d.named) {
			for _, target := range a.namedTargets(f.Type()) {
				key := target.key()

				if agg, isID := a.aggById[key]; isID {
					if agg != root.name {
						refs = append(refs, model.Reference{
							From: root.name,
							To:   agg,
							Via:  fmt.Sprintf("%s.%s %s", cur.d.name, f.Name(), a.typeString(cur.d, f.Type())),
						})
					}
					continue
				}
				// 他の集約ルートの実体を直接持つ場合、境界を越えるので中身に含めない。
				if target.isAggregate {
					continue
				}
				// 識別子型は箱にせず、持ち主のフィールド表示に任せる。
				if a.identity[key] {
					continue
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				reached[key] = true

				out.Members = append(out.Members, model.Member{
					Name:   target.name,
					Pkg:    target.pkgPath,
					Pos:    shortPos(target.pos),
					Kind:   a.classify(target),
					Fields: a.fieldsOf(target),
					Depth:  cur.depth + 1,
				})
				queue = append(queue, item{d: target, depth: cur.depth + 1})
			}
		}
	}

	sort.Slice(out.Members, func(i, j int) bool {
		if out.Members[i].Depth != out.Members[j].Depth {
			return out.Members[i].Depth < out.Members[j].Depth
		}
		return out.Members[i].Name < out.Members[j].Name
	})
	return out, refs
}

// idTypeOf は集約ルート自身の ID 型のキーを返す。
func (a *analyzer) idTypeOf(root *declared) (string, bool) {
	for key, agg := range a.aggById {
		if agg == root.name {
			return key, true
		}
	}
	return "", false
}

// classify は Entity か VO かを判定する。
//
// ポインタレシーバのメソッドを持ち、かつ自身の識別子型のフィールドを
// 持つものを Entity とする。値として振る舞う型は VO。
func (a *analyzer) classify(d *declared) model.Kind {
	hasPointerRecv := false
	for i := 0; i < d.named.NumMethods(); i++ {
		sig, ok := d.named.Method(i).Type().(*types.Signature)
		if !ok || sig.Recv() == nil {
			continue
		}
		if _, isPtr := sig.Recv().Type().(*types.Pointer); isPtr {
			hasPointerRecv = true
			break
		}
	}
	if !hasPointerRecv {
		return model.KindVO
	}

	wantID := d.name + "ID"
	for _, f := range structFields(d.named) {
		for _, target := range a.namedTargets(f.Type()) {
			if target.name == wantID {
				return model.KindEntity
			}
		}
	}
	return model.KindVO
}

func (a *analyzer) fieldsOf(d *declared) []model.Field {
	fields := structFields(d.named)
	out := make([]model.Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, model.Field{
			Name: f.Name(),
			Type: a.typeString(d, f.Type()),
		})
	}
	return out
}

// namedTargets は型を剥がして、解析対象内の名前付き型を取り出す。
// map は key と elem の両方をたどる。
func (a *analyzer) namedTargets(t types.Type) []*declared {
	var out []*declared
	seen := map[types.Type]bool{}

	var walk func(types.Type)
	walk = func(t types.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true

		switch v := t.(type) {
		case *types.Pointer:
			walk(v.Elem())
		case *types.Slice:
			walk(v.Elem())
		case *types.Array:
			walk(v.Elem())
		case *types.Chan:
			walk(v.Elem())
		case *types.Map:
			walk(v.Key())
			walk(v.Elem())
		case *types.Named:
			obj := v.Obj()
			if obj.Pkg() == nil || !a.inScope[obj.Pkg().Path()] {
				return
			}
			if d, ok := a.types[obj.Pkg().Path()+"."+obj.Name()]; ok {
				out = append(out, d)
			}
		}
	}
	walk(t)
	return out
}

func structFields(named *types.Named) []*types.Var {
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	out := make([]*types.Var, 0, st.NumFields())
	for i := 0; i < st.NumFields(); i++ {
		out = append(out, st.Field(i))
	}
	return out
}

// typeString は表示用の型名を作る。同一パッケージの型はパッケージ名を省く。
func (a *analyzer) typeString(from *declared, t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p.Path() == from.pkgPath {
			return ""
		}
		return p.Name()
	})
}

func shortPos(p token.Position) string {
	return fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
}
