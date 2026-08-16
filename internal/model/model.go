// Package model は解析結果の中立表現を定義する。
//
// analyze はこの型のグラフを組み立てて終わり、render はそれを描くだけ。
// 両者はこの JSON を挟んで完全に分離する。
package model

// Graph は解析対象全体の集約構造を表す。
type Graph struct {
	Meta         Meta           `json:"meta"`
	Aggregates   []Aggregate    `json:"aggregates"`
	References   []Reference    `json:"references"`
	Unclassified []Unclassified `json:"unclassified"`
}

// Meta は図の見出しに使う情報。
type Meta struct {
	Title string `json:"title"`
	// Packages は解析対象になったパッケージのパス。
	Packages []string `json:"packages"`
}

// Aggregate は //ddd:aggregate が付いた型と、そこから到達できる中身。
type Aggregate struct {
	Name string `json:"name"`
	Pkg  string `json:"pkg"`
	Pos  string `json:"pos"`
	// IDType は この集約の識別子型の名前。無ければ空。
	IDType  string   `json:"idType,omitempty"`
	Members []Member `json:"members"`
	// Fields は集約ルート自身のフィールド。
	Fields []Field `json:"fields"`
}

// Kind は集約の中身の分類。
type Kind string

const (
	KindEntity Kind = "entity"
	KindVO     Kind = "vo"
)

// Member は集約ルートから到達できる型。
//
// 同じ型が複数の集約から到達可能なら、それぞれの集約に重複して現れる。
// 共有ノードにすると枠をまたぐ辺が生まれ、境界という図の主題がぼやけるため。
type Member struct {
	Name   string  `json:"name"`
	Pkg    string  `json:"pkg"`
	Pos    string  `json:"pos"`
	Kind   Kind    `json:"kind"`
	Fields []Field `json:"fields"`
	// Depth はルートからの最短到達段数。1 が直接のフィールド。
	Depth int `json:"depth"`
}

// Field は構造体のフィールド一つ。
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Reference は集約から集約への ID 参照。
type Reference struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Via は参照が生まれた場所。"Program.exercises []ExerciseID" の形。
	Via string `json:"via"`
}

// Unclassified はどの集約ルートからも到達できなかった型。
//
// 多くはドメインサービスや DTO で正常だが、//ddd:aggregate の
// 付け忘れもここに現れる。
type Unclassified struct {
	Name string `json:"name"`
	Pkg  string `json:"pkg"`
	Pos  string `json:"pos"`
}
