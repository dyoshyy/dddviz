// Package shop covers the cases where classification is easy to get wrong.
package shop

//ddd:aggregate
type Order struct {
	id       OrderID
	customer CustomerID
	lines    []OrderLine
	shipment *Shipment
	total    Money
}

type OrderID string

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
