// Package model defines the neutral representation of an analysis result.
//
// analyze builds this graph and stops there; render only draws it.
// The two are separated by this JSON and never reach across it.
package model

// Graph is the aggregate structure of everything that was analyzed.
type Graph struct {
	Meta       Meta        `json:"meta"`
	Aggregates []Aggregate `json:"aggregates"`
	References []Reference `json:"references"`
	// Services are the types outside every aggregate that work on one.
	// They are drawn beside the aggregates rather than listed as leftovers.
	Services     []Service      `json:"services,omitempty"`
	Unclassified []Unclassified `json:"unclassified"`
	// UnclassifiedTotal counts every unclassified type, including the ones
	// folded into Members. The top-level list is short by design; this keeps
	// the full size visible.
	UnclassifiedTotal int `json:"unclassifiedTotal"`
	// Candidates are unmarked types that own an identifier type.
	//
	// Owning an ID is too weak to decide what an aggregate root is — that
	// is exactly why the marker exists — but it is a reasonable place for
	// someone who has not marked anything yet to start.
	Candidates []Candidate `json:"candidates,omitempty"`
}

// Candidate is a suggested place to put a //ddd:aggregate marker.
type Candidate struct {
	Name string `json:"name"`
	Pkg  string `json:"pkg"`
	Pos  string `json:"pos"`
	// IDType is the identifier type that prompted the suggestion.
	IDType string `json:"idType"`
}

// Meta carries the information shown in the diagram's heading.
type Meta struct {
	Title string `json:"title"`
	// Packages lists the import paths that were analyzed.
	Packages []string `json:"packages"`
	// DomainPackages lists the packages that hold aggregates. Types outside
	// them are not part of the model and are left out of the diagram.
	DomainPackages []string `json:"domainPackages,omitempty"`
}

// Aggregate is a type marked with //ddd:aggregate together with
// everything reachable from it.
type Aggregate struct {
	Name string `json:"name"`
	Pkg  string `json:"pkg"`
	Pos  string `json:"pos"`
	// IDType is the name of this aggregate's identifier type, if any.
	IDType  string   `json:"idType,omitempty"`
	Members []Member `json:"members"`
	// Fields are the fields of the aggregate root itself.
	Fields []Field `json:"fields"`
	// Methods are the root's exported methods.
	Methods []Method `json:"methods,omitempty"`
	// Values are the typed constants declared for this type, in the order
	// they are declared.
	Values []EnumValue `json:"values,omitempty"`
	// Invariants are the rules enforced when the type is built.
	Invariants []Invariant `json:"invariants,omitempty"`
	// Doc is the type's doc comment, with any //ddd: markers removed.
	Doc string `json:"doc,omitempty"`
}

// Kind classifies what lives inside an aggregate.
type Kind string

const (
	KindEntity Kind = "entity"
	KindVO     Kind = "vo"
)

// Member is a type reachable from an aggregate root.
//
// A type reachable from several aggregates appears once under each of them.
// Sharing one node would draw edges across boundaries and blur the very
// thing the diagram is about.
type Member struct {
	Name       string      `json:"name"`
	Pkg        string      `json:"pkg"`
	Pos        string      `json:"pos"`
	Kind       Kind        `json:"kind"`
	Fields     []Field     `json:"fields"`
	Methods    []Method    `json:"methods,omitempty"`
	Values     []EnumValue `json:"values,omitempty"`
	Invariants []Invariant `json:"invariants,omitempty"`
	Doc        string      `json:"doc,omitempty"`
	// Depth is the shortest distance from the root. 1 means a direct field.
	Depth int `json:"depth"`
}

// Field is a single struct field.
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// EnumValue is one typed constant of a named type.
//
// A type like ExerciseKind is not really "a string" — it is the three
// values it can hold, and those values are the domain's vocabulary. The
// type name alone says nothing.
type EnumValue struct {
	Name string `json:"name"`
	// Value is the literal, carried only when it differs from the constant
	// name in more than casing and punctuation.
	Value string `json:"value,omitempty"`
	Doc   string `json:"doc,omitempty"`
}

// Invariant is a rule the type refuses to be constructed without.
//
// These are read off the errors a constructor or a validating method
// returns. A domain model's rules are usually its most important content
// and the part a type declaration says least about.
type Invariant struct {
	Text string `json:"text"`
	Pos  string `json:"pos"`
	// From is the function that enforces the rule.
	From string `json:"from,omitempty"`
}

// Method is an exported method, which is how the rest of the code is allowed
// to use the type. Unexported methods are internal wiring and are left out.
type Method struct {
	Name string `json:"name"`
	// Signature is the parameter and result list, as "(id OrderID) bool".
	Signature string `json:"signature"`
	// Doc is the method's doc comment, if it has one.
	Doc string `json:"doc,omitempty"`
	// Pointer reports a pointer receiver. It is worth showing when a type
	// mixes the two forms; on its own it does not mean the method mutates
	// anything, since read-only methods often take a pointer for consistency.
	Pointer bool `json:"pointer,omitempty"`
}

// Reference is an ID reference from one aggregate to another.
type Reference struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Via is where the reference comes from, as "Order.customer CustomerID".
	Via string `json:"via"`
}

// UnclassifiedKind is what a type's structure says about it.
//
// These come from the type system rather than from guesswork about intent.
// An interface is an interface; a struct with no fields has no state to
// carry. Naming them lets a reader dismiss whole groups at a glance and
// spend attention on KindOther, where the surprises are.
type UnclassifiedKind string

const (
	// KindInterface is an interface — typically a port or repository.
	KindInterface UnclassifiedKind = "interface"
	// KindService is a struct with no fields, which can only be behaviour.
	KindService UnclassifiedKind = "service"
	// KindData is a struct whose fields are all exported, the shape of a
	// type meant to be filled in from outside.
	KindData UnclassifiedKind = "data"
	// KindValue is a named type over something other than a struct.
	KindValue UnclassifiedKind = "value"
	// KindOther is everything else, and the group worth reading.
	KindOther UnclassifiedKind = "other"
)

// Service is a type that belongs to no aggregate but operates on one.
//
// Domain services, policies and repository ports all land here. What
// separates them from stray types is not their shape -- a policy object
// holding configuration looks like any other struct -- but the fact that
// their methods take or return an aggregate.
type Service struct {
	Name string           `json:"name"`
	Pkg  string           `json:"pkg"`
	Pos  string           `json:"pos"`
	Kind UnclassifiedKind `json:"kind"`
	Doc  string           `json:"doc,omitempty"`
	// Touches names the aggregates this type works on.
	Touches []string `json:"touches"`
	Methods []Method `json:"methods,omitempty"`
}

// Unclassified is a type that no aggregate root can reach.
//
// Only types that nothing else unclassified reaches appear at the top
// level; the rest are folded into Members. A domain service pulling in six
// helpers should read as one entry, not seven.
type Unclassified struct {
	Name string           `json:"name"`
	Pkg  string           `json:"pkg"`
	Pos  string           `json:"pos"`
	Kind UnclassifiedKind `json:"kind"`
	// Touches names the aggregates this type takes or returns, which says
	// more about its role than its structure does.
	Touches []string `json:"touches,omitempty"`
	// Members are the unclassified types reachable from this one.
	Members []UnclassifiedRef `json:"members,omitempty"`
}

// UnclassifiedRef is an unclassified type folded under another one.
type UnclassifiedRef struct {
	Name    string           `json:"name"`
	Pkg     string           `json:"pkg"`
	Pos     string           `json:"pos"`
	Kind    UnclassifiedKind `json:"kind"`
	Touches []string         `json:"touches,omitempty"`
	// Depth is the shortest distance from the entry it is folded under.
	Depth int `json:"depth"`
}
