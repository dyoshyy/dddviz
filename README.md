# dddviz

Draw a map of the aggregates in a Go DDD codebase.

![A domain layer drawn by dddviz](docs/diagram.png)

> **The name is a placeholder.** It will be reconsidered once the tool has
> been lived with for a while.

## What it does

The only thing you write in your code is `//ddd:aggregate`, on aggregate
roots. Everything else — what an aggregate contains, which types are
entities and which are value objects, how identifier types pair up, and how
aggregates reference each other — is inferred.

```go
//ddd:aggregate
type Order struct {
	id       OrderID
	customer CustomerID
	lines    []OrderLine
	total    Money
}
```

```console
$ dddviz -C ~/repos/myapp -o docs/domain.html ./internal/...
wrote docs/domain.html (3 aggregates, 2 references, 23 unclassified)
```

The output is one HTML file with no dependencies. Aggregates become frames,
their contents nest inside, and ID references connect them. Each box carries
the type's doc comment, its fields, and its exported methods with their
signatures — enough to read what a type is and how you are meant to use it
without opening the file.

- Click an aggregate to **expand or collapse** it. Collapsed, the diagram is
  a map between aggregates; expanded, it shows what is inside. These are two
  zoom levels of one diagram rather than two separate diagrams
- Hover to **highlight what a type relates to** and dim the rest
- Drag to pan, wheel to zoom, `f` to fit on screen
- Hover a truncated doc line or a method to read the full comment

Use `-format json` to get just the analysis.

### Starting from nothing

Run it before marking anything and it will say so, and point at the types
that at least own an ID type:

```console
$ dddviz -C ~/repos/myapp ./internal/...

dddviz: no aggregates found -- nothing is marked with //ddd:aggregate

  Candidates, from types that own an ID type:
    Exercise                 exercise.go:96         (ExerciseID)
    SetLog                   set_log.go:55          (SetLogID)

  Owning an ID does not make a type an aggregate root, so treat
  these as a starting point rather than an answer.
```

## Watching

```console
$ dddviz -watch -C ~/repos/myapp ./internal/...
dddviz: serving http://127.0.0.1:41533 (3 aggregates, 2 references)
dddviz: watching /home/you/repos/myapp -- press Ctrl-C to stop
```

This serves the diagram, opens a browser, and redraws whenever the code
changes. `-port` fixes the port, `-no-open` skips the browser.

The server pushes a new diagram rather than reloading the page, so **the
aggregates you had expanded stay expanded** across an edit. While the code
does not compile — which is most of the time while you are typing — the last
good diagram stays on screen and the build error appears in a banner.

No file-watching or WebSocket library is involved. `go install` is the whole
setup, and keeping it that way is worth more than either dependency.

## Why one marker is necessary

"An aggregate root is a root of the reference graph, so it can be inferred" —
in practice this does not hold.

Trying "types nothing references are roots" against a real codebase turned up
constructor-argument DTOs, domain services and request/response DTOs, and not
one actual aggregate root.

The reason is structural: in Go it is **aggregate roots that get referenced**,
by ID, so their reference count is never zero. The types with no references
are the DTOs and the services. The hypothesis is backwards.

"Types that own an ID type are roots" fails differently — it misses a
`Program` that has no `ProgramID`.

So dddviz asks the human about the one thing that cannot be inferred, and
infers the rest.

## What is inferred

| Inferred | How |
|---|---|
| Aggregate contents | Types reachable by following fields from the root, minus other aggregate roots |
| Entity vs. value object | Pointer receivers plus a field of the type's own identifier type means entity |
| Identifier pairing | An `Order` marked `//ddd:aggregate` pairs with `OrderID` |
| References between aggregates | A field in aggregate A holding B's ID type means A → B |
| Unclassified | Types no aggregate root can reach. Grouped by what their structure says — interfaces, all-exported structs, fieldless structs — and folded so a service and its helpers read as one entry |
| Methods | Exported methods with their signatures, and their doc comments on hover |

`//ddd:id for=Order` states the pairing when naming departs from the convention.

The marker is prefixed `ddd:` rather than with the tool's name. It goes into
your source, so it should stay neutral rather than tie the code to one tool.

## Why the page is one file

The layout engine and the rendering code are bundled into the binary, which
puts a generated page at around 90KB. Precomputing the layout would remove
the need for JS entirely, but every expand and collapse relayouts, so that
would mean storing coordinates for every combination of open frames.

Nothing is fetched from a CDN and no stylesheet is external, so the output
can be committed straight into a repository without dragging anything else
along.

## Layout

Layout is two different problems, so it is solved in two places.

**Inside an aggregate** the members have no edges between them, so there is
nothing for a graph engine to solve — it is rectangle packing. Packing it
directly keeps full control of the header space a type's name, doc, fields
and methods need.

**Between aggregates** there are edges, ranks and crossings, which is what
[dagre](https://github.com/dagrejs/dagre) is for. dagre returns splines;
they are squared off into right-angled segments before drawing.

An earlier version used elkjs, whose hierarchical layout handled both at
once. It also weighed 1.6MB — around 17 times dagre — for capability this
diagram never needed, since the nested half has no edges to route.

## Working on the rendering code

The page's JavaScript is built from TypeScript in `web/`, and the bundle is
committed. Using dddviz needs only Go; Node is needed to change how the
diagram is drawn, not to draw it.

```console
$ cd web && npm install
$ npm run check     # typecheck against the model types
$ npm run build     # or: go generate ./internal/render
```

`web/src/model.ts` mirrors `internal/model/model.go`. Change a field on the
Go side and the typechecker will point at everything that needs updating.

The figures in this README come from the same rendering code, run under
jsdom rather than captured from a browser, so they stay sharp and stay in
step with the tool:

```console
$ dddviz -C ~/repos/myapp -format json ./internal/... > /tmp/graph.json
$ cd web && npm run shot -- /tmp/graph.json /tmp/diagram.svg --expand
$ rsvg-convert -w 1500 /tmp/diagram.svg -o docs/diagram.png
```

## Install

```console
$ go install github.com/dyoshyy/dddviz/cmd/dddviz@latest
```

## License

MIT. See [LICENSE](LICENSE).

## Status

Verified against a real Go DDD domain layer for both analysis and rendering.

Not addressed yet:

- Behavioural flow (use case → repository call chains) as a first-class view.
  The folded unclassified list already shows its skeleton as a side effect
- Layering violations
