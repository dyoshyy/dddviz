// Package analyze reads Go source and builds a model.Graph of the
// aggregate structure.
//
// The guiding idea is to infer everything that can be inferred and ask
// the human only about the one thing that cannot. A human writes a single
// //ddd:aggregate line; what an aggregate contains, whether a type is an
// entity or a value object, which type is an identifier, and how
// aggregates reference each other all come from the code.
package analyze

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/dyoshyy/dddviz/internal/model"
)

const (
	markerAggregate = "ddd:aggregate"
	markerID        = "ddd:id"
)

// Load analyzes the packages matching patterns, rooted at dir.
func Load(dir string, patterns ...string) (*model.Graph, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir: dir,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}
	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, e.Error())
		}
	})
	if len(loadErrs) > 0 {
		return nil, fmt.Errorf("the code under analysis has errors:\n%s", strings.Join(loadErrs, "\n"))
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages matched: %v", patterns)
	}

	a := &analyzer{
		fset:       pkgs[0].Fset,
		types:      map[string]*declared{},
		inScope:    map[string]bool{},
		aggById:    map[string]string{},
		identity:   map[string]bool{},
		aggregate:  map[string]*declared{},
		enums:      map[string][]model.EnumValue{},
		invariants: map[string][]model.Invariant{},
	}
	for _, p := range pkgs {
		a.collect(p)
	}
	a.resolveIDTypes()
	a.resolveIdentityTypes()
	return a.build(), nil
}

// declared is one named type declared in an analyzed package.
type declared struct {
	named   *types.Named
	pkgPath string
	pkgName string
	name    string
	pos     token.Position
	// isAggregate reports whether //ddd:aggregate is attached.
	isAggregate bool
	// idFor is the X in //ddd:id for=X, empty when absent.
	idFor string
	// doc is the type's doc comment with the //ddd: lines taken out.
	doc string
	// methodDocs maps a method name to its doc comment.
	methodDocs map[string]string
}

func (d *declared) key() string { return d.pkgPath + "." + d.name }

type analyzer struct {
	fset *token.FileSet
	// types holds every named type declared under analysis, keyed by pkgPath.Name.
	types map[string]*declared
	// inScope holds the import paths being analyzed.
	inScope map[string]bool
	// aggById maps an aggregate ID type's key to the aggregate it identifies.
	aggById map[string]string
	// identity holds identifier type keys: aggregate IDs plus entity IDs.
	// Drawing them as boxes adds nothing, so they appear neither as
	// members nor in the unclassified list.
	identity map[string]bool
	// aggregate maps an aggregate root's key to its declaration.
	aggregate map[string]*declared
	// enums maps a type key to its typed constants, in declaration order.
	enums map[string][]model.EnumValue
	// invariants maps a type key to the rules its constructors enforce.
	invariants map[string][]model.Invariant
}

func (a *analyzer) collect(p *packages.Package) {
	a.inScope[p.PkgPath] = true

	markers := map[string][]string{} // type name -> marker lines
	docs := map[string]string{}      // type name -> doc comment
	methodDocs := map[string]map[string]string{}
	constDocs := map[string]string{} // constant name -> doc comment

	for _, syn := range p.Syntax {
		for _, decl := range syn.Decls {
			// Method doc comments live on the function declaration.
			if fd, ok := decl.(*ast.FuncDecl); ok {
				if recv := receiverTypeName(fd); recv != "" && fd.Doc != nil {
					if methodDocs[recv] == nil {
						methodDocs[recv] = map[string]string{}
					}
					methodDocs[recv][fd.Name.Name] = docText(fd.Doc)
				}
				a.collectInvariants(p, fd)
				continue
			}
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			if gd.Tok == token.CONST {
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					// A trailing comment is the usual place to explain a
					// constant, so take it when there is no doc block.
					doc := docText(vs.Doc)
					if doc == "" {
						doc = docText(vs.Comment)
					}
					if doc == "" {
						continue
					}
					for _, n := range vs.Names {
						constDocs[n.Name] = doc
					}
				}
				continue
			}
			if gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				doc := ts.Doc
				// For a standalone declaration the doc sits on the GenDecl.
				// Only borrow it when the block holds a single spec, so a
				// grouped declaration's doc is not handed to every type in it.
				if doc == nil && len(gd.Specs) == 1 {
					doc = gd.Doc
				}
				if ms := parseMarkers(doc); len(ms) > 0 {
					markers[ts.Name.Name] = append(markers[ts.Name.Name], ms...)
				}
				if text := docText(doc); text != "" {
					docs[ts.Name.Name] = text
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
			named:      named,
			pkgPath:    p.PkgPath,
			pkgName:    p.Name,
			name:       name,
			pos:        a.fset.Position(obj.Pos()),
			doc:        docs[name],
			methodDocs: methodDocs[name],
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

	a.collectEnums(p, constDocs)
}

// collectEnums gathers the typed constants declared for each named type.
//
// A type whose values are enumerated is defined as much by that list as by
// its underlying kind, and the list is the vocabulary the domain speaks in.
func (a *analyzer) collectEnums(p *packages.Package, docs map[string]string) {
	scope := p.Types.Scope()

	type entry struct {
		key string
		val model.EnumValue
		pos token.Pos
	}
	var found []entry

	for _, name := range scope.Names() {
		c, ok := scope.Lookup(name).(*types.Const)
		if !ok || !c.Exported() {
			continue
		}
		named, ok := c.Type().(*types.Named)
		if !ok || named.Obj().Pkg() == nil {
			continue
		}
		if !a.inScope[named.Obj().Pkg().Path()] {
			continue
		}

		literal := strings.Trim(c.Val().String(), `"`)
		value := literal
		// Carrying "CHEST_UPPER" beside ChestUpper, or "MAIN" beside
		// KindMain, is noise. Keep the literal only when it says something
		// the constant name does not.
		if nameCovers(name, literal) {
			value = ""
		}

		found = append(found, entry{
			key: named.Obj().Pkg().Path() + "." + named.Obj().Name(),
			val: model.EnumValue{Name: name, Value: value, Doc: docs[name]},
			pos: c.Pos(),
		})
	}

	// Declaration order usually carries meaning that alphabetical order loses.
	sort.Slice(found, func(i, j int) bool { return found[i].pos < found[j].pos })
	for _, e := range found {
		a.enums[e.key] = append(a.enums[e.key], e.val)
	}
}

// nameCovers reports whether a constant's name already carries its literal,
// ignoring case and separators. ChestUpper covers "CHEST_UPPER", and
// KindMain covers "MAIN" because Go constants often repeat the type name as
// a prefix.
func nameCovers(name, literal string) bool {
	norm := func(s string) string {
		var out []rune
		for _, r := range s {
			if r == '_' || r == '-' || r == ' ' {
				continue
			}
			out = append(out, unicode.ToLower(r))
		}
		return string(out)
	}
	n, l := norm(name), norm(literal)
	return l != "" && strings.Contains(n, l)
}

// collectInvariants reads the rules a function refuses to build without.
//
// A function counts when it either has a receiver, or returns a named type
// alongside an error -- that is, when it is the way a value of that type
// comes into existence. Helpers that only return an error belong to no type
// and are skipped; their rules surface through whoever calls them.
func (a *analyzer) collectInvariants(p *packages.Package, fd *ast.FuncDecl) {
	owner := a.invariantOwner(p, fd)
	if owner == "" {
		return
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch {
		case pkg.Name == "errors" && sel.Sel.Name == "New":
		case pkg.Name == "fmt" && sel.Sel.Name == "Errorf":
		default:
			return true
		}

		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		text := ruleText(lit.Value)
		if text == "" {
			return true
		}

		a.invariants[owner] = append(a.invariants[owner], model.Invariant{
			Text: text,
			Pos:  shortPos(a.fset.Position(lit.Pos())),
			From: fd.Name.Name,
		})
		return true
	})
}

// invariantOwner decides which type a function's rules belong to.
func (a *analyzer) invariantOwner(p *packages.Package, fd *ast.FuncDecl) string {
	obj, ok := p.TypesInfo.Defs[fd.Name].(*types.Func)
	if !ok {
		return ""
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return ""
	}

	// A method's rules belong to its receiver.
	if recv := sig.Recv(); recv != nil {
		if named := namedOf(recv.Type()); named != nil {
			return keyOf(named)
		}
		return ""
	}

	// A constructor returns the type it builds, together with an error.
	results := sig.Results()
	if results == nil || results.Len() < 2 {
		return ""
	}
	last := results.At(results.Len() - 1)
	if last.Type().String() != "error" {
		return ""
	}
	for i := 0; i < results.Len()-1; i++ {
		if named := namedOf(results.At(i).Type()); named != nil {
			return keyOf(named)
		}
	}
	return ""
}

// namedOf peels a pointer to reach a named type.
func namedOf(t types.Type) *types.Named {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return nil
	}
	return named
}

func keyOf(named *types.Named) string {
	return named.Obj().Pkg().Path() + "." + named.Obj().Name()
}

// trimSharedPrefix drops the lead-in that every rule of a type repeats.
//
// Errors usually name the thing that failed before stating what was wrong
// with it -- "種目 %s: メイン種目は..." -- which is useful when reading a
// failure and pure noise when reading nine rules of the same type in a row.
func trimSharedPrefix(inv []model.Invariant) []model.Invariant {
	if len(inv) < 2 {
		return inv
	}

	prefix := []rune(inv[0].Text)
	for _, i := range inv[1:] {
		r := []rune(i.Text)
		n := 0
		for n < len(prefix) && n < len(r) && prefix[n] == r[n] {
			n++
		}
		prefix = prefix[:n]
	}
	// Only worth doing when the shared lead-in actually names something.
	if len(prefix) < 2 {
		return inv
	}

	out := make([]model.Invariant, len(inv))
	for k, i := range inv {
		text := strings.TrimLeft(string([]rune(i.Text)[len(prefix):]), ": 　の")
		if text == "" {
			// Nothing but the prefix; keep the original rather than blank it.
			text = i.Text
		}
		i.Text = text
		out[k] = i
	}
	return out
}

// ruleText turns an error format string into a readable rule, or returns
// empty for a string that states no rule.
//
// A format containing %w only wraps someone else's error -- it adds context
// for whoever reads the failure, not a rule of its own, and the rule it
// wraps is already collected where it is stated. The trailing ": %v" that
// carries the offending value is dropped for the same reason.
//
// Other verbs are kept. "レップ数は1〜%dの範囲である必要がある" loses its
// meaning without the %d, since the bound is the rule.
func ruleText(quoted string) string {
	text, err := strconv.Unquote(quoted)
	if err != nil {
		return ""
	}
	if strings.Contains(text, "%w") {
		return ""
	}
	text = strings.TrimSpace(text)
	if i := strings.LastIndex(text, ": %"); i > 0 && len(text)-i <= 4 {
		text = text[:i]
	}
	return strings.TrimSpace(text)
}

// receiverTypeName returns the type a method is declared on, peeling off a
// pointer receiver. It returns "" for plain functions.
func receiverTypeName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	expr := fd.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	// A generic receiver arrives as Type[T]; the name is the part before it.
	if idx, ok := expr.(*ast.IndexExpr); ok {
		expr = idx.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// docText renders a doc comment as plain text, dropping the //ddd: lines so
// a marker never shows up as documentation.
func docText(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	var lines []string
	for _, c := range doc.List {
		text := strings.TrimPrefix(c.Text, "//")
		if strings.HasPrefix(strings.TrimSpace(text), "ddd:") {
			continue
		}
		lines = append(lines, strings.TrimSpace(text))
	}
	// Trim blank lines that the marker may have left behind.
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// parseMarkers extracts the //ddd: lines from a doc comment.
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

// resolveIDTypes pairs identifier types with their aggregates.
//
// By default an aggregate root X is paired with a type named XID.
// //ddd:id for=X states the pairing when the naming departs from that.
func (a *analyzer) resolveIDTypes() {
	for _, agg := range a.aggregate {
		candidate := agg.pkgPath + "." + agg.name + "ID"
		if _, ok := a.types[candidate]; ok {
			a.aggById[candidate] = agg.name
		}
	}
	// An explicit marker wins over the naming convention.
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

// resolveIdentityTypes collects the identifier types.
//
// A type named XID counts as an identifier when X also exists under
// analysis. This covers aggregate IDs (Order -> OrderID) as well as the
// IDs of entities inside an aggregate (Shipment -> ShipmentID).
// Neither is worth a box of its own.
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

	// Types reached from some aggregate. Drives the unclassified list.
	reached := map[string]bool{}

	for _, agg := range a.aggregate {
		built, refs := a.buildAggregate(agg, reached)
		g.Aggregates = append(g.Aggregates, built)
		g.References = append(g.References, refs...)
	}

	g.Services, g.Unclassified = a.outside(reached)
	for key, d := range a.types {
		if !d.isAggregate && !reached[key] && !a.identity[key] {
			g.UnclassifiedTotal++
		}
	}
	g.Candidates = a.candidates()

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
	// g.Unclassified is already ordered by group and then by name.
	return g
}

// unclassified collects the types no aggregate can reach, groups them by
// what their structure says about them, and folds each one's own reachable
// types underneath it.
//
// A domain service that drags in six helpers should read as one entry, not
// as seven equal names in a flat list.
func (a *analyzer) outside(reached map[string]bool) ([]model.Service, []model.Unclassified) {
	set := map[string]*declared{}
	for key, d := range a.types {
		if d.isAggregate || reached[key] || a.identity[key] {
			continue
		}
		set[key] = d
	}

	// A type whose methods take or return an aggregate is doing work on the
	// domain, so it is drawn beside the aggregates rather than left in a
	// list of things that did not fit.
	var services []model.Service
	for key, d := range set {
		touches := a.touches(d)
		if len(touches) == 0 {
			continue
		}
		services = append(services, model.Service{
			Name:    d.name,
			Pkg:     d.pkgPath,
			Pos:     shortPos(d.pos),
			Kind:    classifyUnclassified(d),
			Doc:     d.doc,
			Touches: touches,
			Methods: a.methodsOf(d),
		})
		delete(set, key)
	}
	sort.Slice(services, func(i, j int) bool {
		if services[i].Touches[0] != services[j].Touches[0] {
			return services[i].Touches[0] < services[j].Touches[0]
		}
		return services[i].Name < services[j].Name
	})

	if len(set) == 0 {
		return services, []model.Unclassified{}
	}

	// Reachability among the unclassified types only.
	edges := map[string][]string{}
	referenced := map[string]bool{}
	for key, d := range set {
		seen := map[string]bool{key: true}
		for _, f := range structFields(d.named) {
			for _, t := range a.namedTargets(f.Type()) {
				k := t.key()
				if _, ok := set[k]; !ok || seen[k] {
					continue
				}
				seen[k] = true
				edges[key] = append(edges[key], k)
				referenced[k] = true
			}
		}
	}

	tops := make([]string, 0, len(set))
	for key := range set {
		if !referenced[key] {
			tops = append(tops, key)
		}
	}
	sort.Strings(tops)

	out := make([]model.Unclassified, 0, len(tops))
	covered := map[string]bool{}
	for _, key := range tops {
		out = append(out, a.foldUnclassified(key, set, edges, covered))
	}

	// Types caught in a cycle are referenced by something, yet nothing
	// outside the cycle reaches them. Surface them rather than lose them.
	var orphans []string
	for key := range set {
		if !covered[key] {
			orphans = append(orphans, key)
		}
	}
	sort.Strings(orphans)
	for _, key := range orphans {
		if covered[key] {
			continue // an earlier orphan already pulled this one in
		}
		out = append(out, a.foldUnclassified(key, set, edges, covered))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return kindOrder(out[i].Kind) < kindOrder(out[j].Kind)
		}
		return out[i].Name < out[j].Name
	})
	return services, out
}

// foldUnclassified walks out from one entry, collecting what it reaches.
func (a *analyzer) foldUnclassified(
	key string,
	set map[string]*declared,
	edges map[string][]string,
	covered map[string]bool,
) model.Unclassified {
	root := set[key]
	covered[key] = true

	entry := model.Unclassified{
		Name:    root.name,
		Pkg:     root.pkgPath,
		Pos:     shortPos(root.pos),
		Kind:    classifyUnclassified(root),
		Touches: a.touches(root),
	}

	seen := map[string]bool{key: true}
	type item struct {
		key   string
		depth int
	}
	queue := []item{{key: key, depth: 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range edges[cur.key] {
			if seen[next] {
				continue
			}
			seen[next] = true
			covered[next] = true

			d := set[next]
			entry.Members = append(entry.Members, model.UnclassifiedRef{
				Name:    d.name,
				Pkg:     d.pkgPath,
				Pos:     shortPos(d.pos),
				Kind:    classifyUnclassified(d),
				Touches: a.touches(d),
				Depth:   cur.depth + 1,
			})
			queue = append(queue, item{key: next, depth: cur.depth + 1})
		}
	}

	sort.Slice(entry.Members, func(i, j int) bool {
		if entry.Members[i].Depth != entry.Members[j].Depth {
			return entry.Members[i].Depth < entry.Members[j].Depth
		}
		return entry.Members[i].Name < entry.Members[j].Name
	})
	return entry
}

// touches lists the aggregates a type reaches through its method signatures.
//
// A type outside every aggregate is still part of the domain when it takes
// or returns one. Naming what it touches says more about its role than its
// shape does -- a policy object and a stateless service look identical from
// the outside.
func (a *analyzer) touches(d *declared) []string {
	seen := map[string]bool{}
	var out []string

	add := func(t types.Type) {
		for _, target := range a.namedTargets(t) {
			key := target.key()
			name := ""
			if target.isAggregate {
				name = target.name
			} else if agg, ok := a.aggById[key]; ok {
				name = agg
			}
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}

	for _, fn := range exportedMethods(d.named) {
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			continue
		}
		for _, list := range []*types.Tuple{sig.Params(), sig.Results()} {
			for j := 0; j < list.Len(); j++ {
				add(list.At(j).Type())
			}
		}
	}

	sort.Strings(out)
	return out
}

// exportedMethods returns a type's exported methods.
//
// An interface keeps its methods on the underlying type rather than on the
// named one, so a repository port would otherwise look like it has none.
func exportedMethods(named *types.Named) []*types.Func {
	var out []*types.Func

	if iface, ok := named.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			if fn := iface.Method(i); fn.Exported() {
				out = append(out, fn)
			}
		}
		return out
	}

	for i := 0; i < named.NumMethods(); i++ {
		if fn := named.Method(i); fn.Exported() {
			out = append(out, fn)
		}
	}
	return out
}

// classifyUnclassified reads a type's structure. It says nothing about
// intent — only about what the type system already states.
func classifyUnclassified(d *declared) model.UnclassifiedKind {
	switch u := d.named.Underlying().(type) {
	case *types.Interface:
		return model.KindInterface
	case *types.Struct:
		if u.NumFields() == 0 {
			return model.KindService
		}
		for i := 0; i < u.NumFields(); i++ {
			if !u.Field(i).Exported() {
				return model.KindOther
			}
		}
		return model.KindData
	default:
		return model.KindValue
	}
}

// kindOrder puts the groups a reader can dismiss first and the group worth
// reading last.
func kindOrder(k model.UnclassifiedKind) int {
	switch k {
	case model.KindInterface:
		return 0
	case model.KindData:
		return 1
	case model.KindService:
		return 2
	case model.KindValue:
		return 3
	default:
		return 4
	}
}

// candidates suggests where a //ddd:aggregate marker might go, for someone
// who has not marked anything yet.
//
// A type is suggested when a matching XID type exists and the type is not
// already marked. This is deliberately not how roots are decided — owning an
// ID misses roots that have none, which is why the marker exists at all —
// but as a starting point for a first pass it saves reading every file.
func (a *analyzer) candidates() []model.Candidate {
	var out []model.Candidate

	for _, d := range a.types {
		if d.isAggregate {
			continue
		}
		idKey := d.pkgPath + "." + d.name + "ID"
		id, ok := a.types[idKey]
		if !ok {
			continue
		}
		// The ID has to look like an identifier, not a struct that merely
		// ends in "ID".
		if _, isStruct := id.named.Underlying().(*types.Struct); isStruct {
			continue
		}
		out = append(out, model.Candidate{
			Name:   d.name,
			Pkg:    d.pkgPath,
			Pos:    shortPos(d.pos),
			IDType: id.name,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// meta builds the information shown in the diagram's heading.
// The title comes from the common prefix of the analyzed packages.
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

// commonPrefix returns the shared leading segments of the given paths.
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

// buildAggregate walks breadth-first over the types reachable from a root.
func (a *analyzer) buildAggregate(root *declared, reached map[string]bool) (model.Aggregate, []model.Reference) {
	out := model.Aggregate{
		Name:       root.name,
		Pkg:        root.pkgPath,
		Pos:        shortPos(root.pos),
		Members:    []model.Member{},
		Fields:     a.fieldsOf(root),
		Methods:    a.methodsOf(root),
		Values:     a.enums[root.key()],
		Invariants: trimSharedPrefix(a.invariants[root.key()]),
		Doc:        root.doc,
	}
	if key, ok := a.idTypeOf(root); ok {
		out.IDType = a.types[key].name
	}

	var refs []model.Reference
	// seen prevents revisiting within one aggregate; across aggregates duplicates are intended.
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
				// Holding another root by value crosses a boundary, so it is not a member.
				if target.isAggregate {
					continue
				}
				// Identifier types get no box; the owner's field list already shows them.
				if a.identity[key] {
					continue
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				reached[key] = true

				out.Members = append(out.Members, model.Member{
					Name:       target.name,
					Pkg:        target.pkgPath,
					Pos:        shortPos(target.pos),
					Kind:       a.classify(target),
					Fields:     a.fieldsOf(target),
					Methods:    a.methodsOf(target),
					Values:     a.enums[key],
					Invariants: trimSharedPrefix(a.invariants[key]),
					Doc:        target.doc,
					Depth:      cur.depth + 1,
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

// idTypeOf returns the key of the root's own identifier type.
func (a *analyzer) idTypeOf(root *declared) (string, bool) {
	for key, agg := range a.aggById {
		if agg == root.name {
			return key, true
		}
	}
	return "", false
}

// classify decides between entity and value object.
//
// A type with pointer-receiver methods that also carries a field of its
// own identifier type is an entity. Types that behave as values are VOs.
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

// methodsOf lists a type's exported methods.
//
// Exported methods are the type's contract -- what the rest of the code is
// allowed to do with it. Unexported ones are internal wiring and would only
// crowd the box.
func (a *analyzer) methodsOf(d *declared) []model.Method {
	var out []model.Method

	for _, fn := range exportedMethods(d.named) {
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			continue
		}
		pointer := false
		if recv := sig.Recv(); recv != nil {
			_, pointer = recv.Type().(*types.Pointer)
		}

		out = append(out, model.Method{
			Name: fn.Name(),
			// TypeString renders a signature as "func(a A) B"; the method
			// name takes the place of the "func".
			Signature: strings.TrimPrefix(a.typeString(d, sig), "func"),
			Doc:       d.methodDocs[fn.Name()],
			Pointer:   pointer,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
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

// namedTargets peels a type down to the named types under analysis.
// Both the key and the element of a map are followed.
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

// typeString renders a type for display, dropping the package qualifier
// for types in the same package.
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
