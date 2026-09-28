package crd_test

import (
	"k8s.io/apimachinery/pkg/util/intstr"

	"crd.tools/crd"
)

// commonSpec это поле с общими свойствами
type commonSpec struct {
	Name *string `json:"name"`
}

func (s *commonSpec) CRD(b crd.Builder) error {
	b.String(&s.Name).Nullable().Title("Отображаемое имя").Example("alice").MaxLength(64)
	return nil
}

// Example_commonProps показывает свойства, общие для всех правил
func Example_commonProps() {
	printSchema[commonSpec]()
	// Output:
	// properties:
	//   name:
	//     example: alice
	//     maxLength: 64
	//     nullable: true
	//     title: Отображаемое имя
	//     type: string
	// required:
	// - name
	// type: object
}

// celSpec это поля со связью через CEL
type celSpec struct {
	MinReplicas int `json:"minReplicas"`
	MaxReplicas int `json:"maxReplicas"`
}

func (s *celSpec) CRD(b crd.Builder) error {
	b.Integer(&s.MinReplicas).Minimum(0)
	b.Integer(&s.MaxReplicas).Minimum(0)
	b.Self().XValidation("self.minReplicas <= self.maxReplicas", "minReplicas must not exceed maxReplicas")
	return nil
}

// Example_celValidation показывает CEL-правило на объекте для связи между полями
func Example_celValidation() {
	printSchema[celSpec]()
	// Output:
	// properties:
	//   maxReplicas:
	//     minimum: 0
	//     type: integer
	//   minReplicas:
	//     minimum: 0
	//     type: integer
	// required:
	// - maxReplicas
	// - minReplicas
	// type: object
	// x-kubernetes-validations:
	// - message: minReplicas must not exceed maxReplicas
	//   rule: self.minReplicas <= self.maxReplicas
}

// intOrStringSpec это номер или имя порта
type intOrStringSpec struct {
	Port    intstr.IntOrString `json:"port"`
	Timeout string             `json:"timeout"`
}

func (s *intOrStringSpec) CRD(b crd.Builder) error {
	b.IntOrString(&s.Port).Description("номер или имя порта")
	b.IntOrString(&s.Timeout).Description("секунды или длительность")
	return nil
}

// Example_intOrString показывает поле, которое может быть целым числом или строкой
// intstr.IntOrString распознаётся сам, для строкового поля правило задаётся явно
func Example_intOrString() {
	printSchema[intOrStringSpec]()
	// Output:
	// properties:
	//   port:
	//     description: номер или имя порта
	//     x-kubernetes-int-or-string: true
	//   timeout:
	//     description: секунды или длительность
	//     x-kubernetes-int-or-string: true
	// required:
	// - port
	// - timeout
	// type: object
}

// numberEnumSpec это число из фиксированного набора
type numberEnumSpec struct {
	Ratio float64 `json:"ratio"`
}

func (s *numberEnumSpec) CRD(b crd.Builder) error {
	b.Number(&s.Ratio).Enum(0.25, 0.5, 1.0)
	return nil
}

// Example_numberEnum показывает число с плавающей точкой из фиксированного набора
func Example_numberEnum() {
	printSchema[numberEnumSpec]()
	// Output:
	// properties:
	//   ratio:
	//     enum:
	//     - 0.25
	//     - 0.5
	//     - 1
	//     format: double
	//     type: number
	// required:
	// - ratio
	// type: object
}

// rulerClientID описывает свою схему как тип-значение
type rulerClientID string

func (rulerClientID) Rule() (crd.Rule, error) {
	return crd.K8sName().MaxLength(64).Description("идентификатор клиента"), nil
}

// rulerSpec использует тип с Ruler в двух полях
type rulerSpec struct {
	Owner  rulerClientID `json:"owner"`
	Viewer rulerClientID `json:"viewer,omitempty"`
}

func (s *rulerSpec) CRD(b crd.Builder) error {
	b.String(&s.Viewer).Description("кто может смотреть")
	return nil
}

// Example_ruler показывает тип, который описывает свою схему сам
// Схема типа переиспользуется во всех полях, описание поля накладывается поверх
func Example_ruler() {
	printSchema[rulerSpec]()
	// Output:
	// properties:
	//   owner:
	//     description: идентификатор клиента
	//     maxLength: 64
	//     pattern: ^[a-z0-9-]+$
	//     type: string
	//   viewer:
	//     description: кто может смотреть
	//     maxLength: 64
	//     pattern: ^[a-z0-9-]+$
	//     type: string
	// required:
	// - owner
	// type: object
}
