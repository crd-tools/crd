package crd_test

import (
	"crd.tools/crd"
)

// oneOfSpec это строка одного из двух видов
type oneOfSpec struct {
	Ref string `json:"ref"`
}

func (s *oneOfSpec) CRD(b crd.Builder) error {
	ref := b.String(&s.Ref)
	ref.OneOf().MaxLength(8).Pattern(`^[a-z]+$`)
	ref.OneOf().Pattern(`^[0-9a-f]{32}$`)
	return nil
}

// Example_oneOfString показывает строку, которая подходит ровно под одну из веток
// Ветки не несут тип, тип остаётся на самом поле
func Example_oneOfString() {
	printSchema[oneOfSpec]()
	// Output:
	// properties:
	//   ref:
	//     oneOf:
	//     - maxLength: 8
	//       pattern: ^[a-z]+$
	//     - pattern: ^[0-9a-f]{32}$
	//     type: string
	// required:
	// - ref
	// type: object
}

// anyOfSpec это целое в одном из диапазонов
type anyOfSpec struct {
	Size int `json:"size"`
}

func (s *anyOfSpec) CRD(b crd.Builder) error {
	size := b.Integer(&s.Size)
	size.AnyOf().Minimum(2).Maximum(10)
	size.AnyOf().Minimum(12).Maximum(80)
	size.AnyOf().Minimum(100)
	return nil
}

// Example_anyOfInteger показывает целое, допустимое в любом из диапазонов
func Example_anyOfInteger() {
	printSchema[anyOfSpec]()
	// Output:
	// properties:
	//   size:
	//     anyOf:
	//     - maximum: 10
	//       minimum: 2
	//     - maximum: 80
	//       minimum: 12
	//     - minimum: 100
	//     type: integer
	// required:
	// - size
	// type: object
}

// allOfSpec это строка, которая должна подойти под все ветки
type allOfSpec struct {
	Code string `json:"code"`
}

func (s *allOfSpec) CRD(b crd.Builder) error {
	code := b.String(&s.Code)
	code.AllOf().Pattern(`^[a-z]`)
	code.AllOf().MaxLength(8)
	return nil
}

// Example_allOfString показывает строку, которая должна подойти под все ветки сразу
func Example_allOfString() {
	printSchema[allOfSpec]()
	// Output:
	// properties:
	//   code:
	//     maxLength: 8
	//     pattern: ^[a-z]
	//     type: string
	// required:
	// - code
	// type: object
}

// notSpec это непустая строка через not
type notSpec struct {
	Name string `json:"name"`
}

func (s *notSpec) CRD(b crd.Builder) error {
	b.String(&s.Name).Not().Enum("")
	return nil
}

// Example_notString показывает строку, которая не должна подойти под ветку
func Example_notString() {
	printSchema[notSpec]()
	// Output:
	// properties:
	//   name:
	//     not:
	//       enum:
	//       - ""
	//     type: string
	// required:
	// - name
	// type: object
}

// oneOfSolver это взаимоисключающие поля
type oneOfSolver struct {
	HTTP01 *string `json:"http01,omitempty"`
	DNS01  *string `json:"dns01,omitempty"`
}

// oneOfObjectSpec это поле с ровно одним из вариантов
type oneOfObjectSpec struct {
	Solver oneOfSolver `json:"solver"`
}

func (s *oneOfObjectSpec) CRD(b crd.Builder) error {
	solver := b.Object(&s.Solver)
	solver.OneOf().RequiredFields("http01")
	solver.OneOf().RequiredFields("dns01")
	return nil
}

// Example_oneOfObject показывает взаимоисключающие поля объекта
// Должно быть ровно одно из полей http01 и dns01, сами поля берутся из Go-типа
func Example_oneOfObject() {
	printSchema[oneOfObjectSpec]()
	// Output:
	// properties:
	//   solver:
	//     oneOf:
	//     - required:
	//       - http01
	//     - required:
	//       - dns01
	//     properties:
	//       dns01:
	//         type: string
	//       http01:
	//         type: string
	//     type: object
	// required:
	// - solver
	// type: object
}
