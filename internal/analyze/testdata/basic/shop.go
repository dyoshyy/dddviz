// Package shop は解析ルールの判定が割れる箇所を一通り含む。
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

// OrderLine は値レシーバのみ。VO として分類されるべき。
type OrderLine struct {
	sku SKU
	qty Quantity
}

func (l OrderLine) SKU() SKU      { return l.sku }
func (l OrderLine) Qty() Quantity { return l.qty }

// Shipment はポインタレシーバと ShipmentID フィールドを持つ。
// ただし //ddd:aggregate が無いので集約ルートではなく、Order の中の Entity。
// ShipmentID も集約 ID 型ではないので参照矢印は生まれない。
type Shipment struct {
	id      ShipmentID
	carrier string
}

type ShipmentID string

func (s *Shipment) ID() ShipmentID { return s.id }

// Money は複数の集約から到達される VO。両方に重複して現れるべき。
type Money struct {
	amount   int64
	currency string
}

func (m Money) Amount() int64 { return m.amount }

// SKU と Quantity は struct でないプリミティブラッパーの VO。
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

// PricingService はどの集約からも到達できない。未分類に出るべき。
type PricingService struct{}

func (PricingService) Price(sku SKU) Money { return Money{} }

// PlaceOrderRequest も到達不能な DTO。
type PlaceOrderRequest struct {
	CustomerID CustomerID
	Lines      []OrderLine
}
