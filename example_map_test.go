package crd_test

import (
	"crd.tools/crd"
)

// mapValuesSpec это мапа со скалярным значением
type mapValuesSpec struct {
	Counts map[string]int `json:"counts"`
}

func (s *mapValuesSpec) CRD(b crd.Builder) error {
	b.Map(&s.Counts).MaxProperties(10).Values(crd.NewInteger().Minimum(0).Maximum(100))
	return nil
}

// Example_mapValues показывает ограничения мапы и её значения
func Example_mapValues() {
	printSchema[mapValuesSpec]()
	// Output:
	// properties:
	//   counts:
	//     additionalProperties:
	//       maximum: 100
	//       minimum: 0
	//       type: integer
	//     maxProperties: 10
	//     type: object
	// required:
	// - counts
	// type: object
}

// mapItem это значение мапы со своим описанием
type mapItem struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func (s *mapItem) CRD(b crd.Builder) error {
	b.String(&s.Name).MinLength(1).MaxLength(64)
	b.String(&s.Kind).Enum("a", "b")
	return nil
}

// mapStructSpec это мапа со значением-структурой
type mapStructSpec struct {
	Items map[string]mapItem `json:"items"`
}

func (s *mapStructSpec) CRD(b crd.Builder) error {
	b.Map(&s.Items).MinProperties(1)
	return nil
}

// Example_mapStructValue показывает мапу, значение которой описано на своём типе
func Example_mapStructValue() {
	printSchema[mapStructSpec]()
	// Output:
	// properties:
	//   items:
	//     additionalProperties:
	//       properties:
	//         kind:
	//           enum:
	//           - a
	//           - b
	//           type: string
	//         name:
	//           maxLength: 64
	//           minLength: 1
	//           type: string
	//       required:
	//       - kind
	//       - name
	//       type: object
	//     minProperties: 1
	//     type: object
	// required:
	// - items
	// type: object
}

// mapTag это элемент среза внутри значения мапы
type mapTag struct {
	Name string `json:"name"`
}

func (s *mapTag) CRD(b crd.Builder) error {
	b.String(&s.Name).MinLength(1)
	return nil
}

// mapNestedSpec это мапа, значение которой — срез структур
type mapNestedSpec struct {
	Tags map[string][]mapTag `json:"tags"`
}

func (s *mapNestedSpec) CRD(b crd.Builder) error {
	b.Map(&s.Tags).MaxProperties(5).Values(crd.NewArray(crd.Of[mapTag]()).MinItems(1))
	return nil
}

// Example_mapNested показывает мапу со значением-срезом структур
// crd.Of ссылается на тип как на правило-значение, чтобы задать ограничения среза
func Example_mapNested() {
	printSchema[mapNestedSpec]()
	// Output:
	// properties:
	//   tags:
	//     additionalProperties:
	//       items:
	//         properties:
	//           name:
	//             minLength: 1
	//             type: string
	//         required:
	//         - name
	//         type: object
	//       minItems: 1
	//       type: array
	//     maxProperties: 5
	//     type: object
	// required:
	// - tags
	// type: object
}
