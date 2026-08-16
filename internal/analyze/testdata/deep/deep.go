// Package deep covers reachability that goes further than one or two hops,
// and shapes that could make the walk loop forever.
package deep

//ddd:aggregate
type Order struct {
	// Three ways of holding a type, all of which must be followed.
	basket   Basket
	lines    []Line
	byRegion map[Region]Discount
}

// A chain four hops deep. Every link has to appear.
type Basket struct{ line Line }
type Line struct{ discount Discount }
type Discount struct{ rule Rule }
type Rule struct{ window Window }
type Window struct{ days int }

type Region string

// Shipment and Leg refer to each other. The walk must terminate.
//
//ddd:aggregate
type Shipment struct{ leg Leg }

type Leg struct {
	// Back to the root, which is a separate aggregate and must not be
	// pulled in as a member.
	shipment *Shipment
	// A second chain, to check depth is measured from the root.
	tracking Tracking
}

type Tracking struct{ carrier Carrier }
type Carrier struct{ name string }
