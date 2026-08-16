// Package shop covers the cases where classification is easy to get wrong.
package shop

// Order is a customer's purchase, and the unit of consistency here.
//
// The second paragraph should survive into the doc text, while the marker
// below must not.
//
//ddd:aggregate
type Order struct {
	id       OrderID
	customer CustomerID
	lines    []OrderLine
	shipment *Shipment
	total    Money
	status   Status
	priority Priority
}

type OrderID string

// Status is what an order can be. The constants are declared out of
// alphabetical order on purpose: declaration order carries the lifecycle.
type Status string

const (
	StatusDraft   Status = "DRAFT"
	StatusPlaced  Status = "PLACED"
	StatusShipped Status = "SHIPPED"
	// StatusVoid ends the order without shipping it.
	StatusVoid Status = "VOID"
)

// Priority values are numbers, so the literal says something the name does not.
type Priority int

const (
	PriorityLow    Priority = 1
	PriorityNormal Priority = 5
	PriorityHigh   Priority = 9
)

// unexportedConst must not appear.
const unexportedConst Status = "HIDDEN"

// ID identifies the order.
func (o *Order) ID() OrderID        { return o.id }
func (o *Order) Total() Money       { return o.total }
func (o *Order) Lines() []OrderLine { return o.lines }

// OrderLine has value receivers only, so it must classify as a VO.
type OrderLine struct {
	sku SKU
	qty Quantity
}

func (l OrderLine) SKU() SKU      { return l.sku }
func (l OrderLine) Qty() Quantity { return l.qty }

// Shipment has a pointer receiver and a ShipmentID field, but no
// //ddd:aggregate marker. It is an entity inside Order rather than a root,
// and ShipmentID is not an aggregate ID, so it draws no reference arrow.
type Shipment struct {
	id      ShipmentID
	carrier string
}

type ShipmentID string

func (s *Shipment) ID() ShipmentID { return s.id }

// Money is reachable from two aggregates and must appear under both.
type Money struct {
	amount   int64
	currency string
}

func (m Money) Amount() int64 { return m.amount }

// Add returns the sum, leaving the receiver alone.
func (m Money) Add(o Money) Money { return Money{m.amount + o.amount, m.currency} }

// Charge mutates through a pointer, so Money mixes receiver forms.
func (m *Money) Charge(n int64) { m.amount += n }

// unexported must not appear in the diagram.
func (m Money) unexported() {}

// SKU and Quantity are value objects that wrap a primitive rather than a struct.
type SKU string

type Quantity int

//ddd:aggregate
type Customer struct {
	id      CustomerID
	name    string
	balance Money
}

type CustomerID string

func (c *Customer) ID() CustomerID { return c.id }

// PricingService is reachable from no aggregate, so it lands in unclassified.
type PricingService struct{}

func (PricingService) Price(sku SKU) Money { return Money{} }

// PlaceOrderRequest is an unreachable DTO.
type PlaceOrderRequest struct {
	CustomerID CustomerID
	Lines      []OrderLine
}
