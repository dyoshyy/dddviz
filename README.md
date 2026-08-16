# dddviz

Go の DDD コードから集約の構造を抽出する。

> **名前は仮です。** POC で手応えを確かめてから考え直します。

## 何をするか

コードに `//ddd:aggregate` を書くのは集約ルートだけ。
それ以外——集約の中身、Entity と VO の別、ID 型の対応づけ、集約間の参照——は
すべてコードから推論します。

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
$ dddviz -C ~/repos/myapp ./internal/...
```

```
[Order] id=OrderID
   d1 vo     Money
   d1 vo     OrderLine
   d1 entity Shipment
[Customer] id=CustomerID
   d1 vo     Money

references
  Order -> Customer   via Order.customer CustomerID

unclassified
  PricingService, PlaceOrderRequest
```

## なぜマーカーが1つ必要なのか

「集約ルートは参照グラフの根だから推論できる」——実際に当てると外れます。

実在のコードベースで「被参照ゼロの型がルート」を試したところ、
検出されたのはコンストラクタ引数の DTO、ドメインサービス、入出力 DTO ばかりで、
本物の集約ルートは一つも含まれませんでした。

Go では**集約ルートこそ ID 経由で他から参照される**ので、被参照数はゼロになりません。
ゼロになるのは DTO とサービスのほうです。仮説がひっくり返っている。

「ID 型を持つ型がルート」という別案も、`ProgramID` を持たない `Program` を取り逃します。

そこで **原理的に推論不能な1点だけ人間に聞き、残りは全部推論する** 方針を取りました。

## 推論しているもの

| 導くもの | 導き方 |
|---|---|
| 集約の中身 | ルートからフィールドの型をたどって到達可能な型のうち、他の集約ルートでないもの |
| Entity か VO か | ポインタレシーバを持ち、かつ自身の識別子型のフィールドを持つものが Entity |
| ID 型の対応づけ | `//ddd:aggregate` の付いた `Order` に対し `OrderID` を自動対応 |
| 集約間の参照 | 集約 A のフィールドに集約 B の ID 型が現れたら A → B |
| 未分類 | どの集約ルートからも到達できない型。サービスや DTO が並ぶが、マーカーの付け忘れもここに出る |

命名が規約から外れる場合だけ `//ddd:id for=Order` で明示できます。

マーカーの接頭辞をツール名ではなく `ddd:` にしているのは、
コードに書き込ませる以上、中立な記法にしてベンダーロックを避けるためです。

## 状態

POC 1（解析部）まで。出力は JSON のみです。

インタラクティブな HTML の描画は POC 2 で作ります。
集約の枠をクリックで展開・折りたたみして、集約間の地図と中身の詳細を
同じ図の粒度切り替えとして扱えるようにする予定です。

## インストール

```console
$ go install github.com/dyoshyy/dddviz/cmd/dddviz@latest
```
