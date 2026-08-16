// Package model defines the neutral representation of an analysis result.
//
// analyze builds this graph and stops there; render only draws it.
// The two are separated by this JSON and never reach across it.
package model

// Graph is the aggregate structure of everything that was analyzed.
type Graph struct {
	Meta         Meta           `json:"meta"`
	Aggregates   []Aggregate    `json:"aggregates"`
	References   []Reference    `json:"references"`
	Unclassified []Unclassified `json:"unclassified"`
}

// Meta carries the information shown in the diagram's heading.
type Meta struct {
	Title string `json:"title"`
	// Packages lists the import paths that were analyzed.
	Packages []string `json:"packages"`
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
	Name   string  `json:"name"`
	Pkg    string  `json:"pkg"`
	Pos    string  `json:"pos"`
	Kind   Kind    `json:"kind"`
	Fields []Field `json:"fields"`
	// Depth is the shortest distance from the root. 1 means a direct field.
	Depth int `json:"depth"`
}

// Field is a single struct field.
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Reference is an ID reference from one aggregate to another.
type Reference struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Via is where the reference comes from, as "Order.customer CustomerID".
	Via string `json:"via"`
}

// Unclassified is a type that no aggregate root can reach.
//
// Most are domain services or DTOs and perfectly fine, but a forgotten
// //ddd:aggregate marker shows up here too.
type Unclassified struct {
	Name string `json:"name"`
	Pkg  string `json:"pkg"`
	Pos  string `json:"pos"`
}
