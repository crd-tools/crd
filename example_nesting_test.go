package crd_test

import (
	"crd.tools/crd"
	"crd.tools/crd/formats"
)

// scalarSpec это скалярные поля
type scalarSpec struct {
	Name    string  `json:"name"`
	Count   int     `json:"count"`
	Ratio   float64 `json:"ratio"`
	Enabled bool    `json:"enabled"`
}

func (s *scalarSpec) CRD(b crd.Builder) error {
	b.String(&s.Name).MinLength(1).MaxLength(128)
	b.Integer(&s.Count).Minimum(0).Maximum(100).Default(1).Optional()
	b.Number(&s.Ratio).Minimum(0).Maximum(1)
	b.Bool(&s.Enabled).Default(true).Optional()
	return nil
}

// Example_scalar показывает строку, целое, число с плавающей точкой и булево значение
// Поля без omitempty обязательны, Optional снимает обязательность
func Example_scalar() {
	printSchema[scalarSpec]()
	// Output:
	// properties:
	//   count:
	//     default: 1
	//     maximum: 100
	//     minimum: 0
	//     type: integer
	//   enabled:
	//     default: true
	//     type: boolean
	//   name:
	//     maxLength: 128
	//     minLength: 1
	//     type: string
	//   ratio:
	//     format: double
	//     maximum: 1
	//     minimum: 0
	//     type: number
	// required:
	// - name
	// - ratio
	// type: object
}

// objectAddress это почтовый адрес без своего описания
type objectAddress struct {
	City string `json:"city"`
	Zip  string `json:"zip"`
}

// objectSpec описывает поля вложенной структуры напрямую
type objectSpec struct {
	Address objectAddress `json:"address"`
}

func (s *objectSpec) CRD(b crd.Builder) error {
	b.Object(&s.Address).Description("почтовый адрес")
	b.String(&s.Address.City).MinLength(1)
	b.String(&s.Address.Zip).Pattern(`^[0-9]{5}$`)
	return nil
}

// Example_object показывает вложенную структуру
// Поля структуры без своего описания адресуются напрямую: &s.Address.City
func Example_object() {
	printSchema[objectSpec]()
	// Output:
	// properties:
	//   address:
	//     description: почтовый адрес
	//     properties:
	//       city:
	//         minLength: 1
	//         type: string
	//       zip:
	//         pattern: ^[0-9]{5}$
	//         type: string
	//     required:
	//     - city
	//     - zip
	//     type: object
	// required:
	// - address
	// type: object
}

// arrayScalarSpec это массив строк
type arrayScalarSpec struct {
	URIs []string `json:"uris"`
}

func (s *arrayScalarSpec) CRD(b crd.Builder) error {
	b.Array(&s.URIs).MinItems(1).Items(crd.HTTPSURL().Format(formats.URI))
	return nil
}

// Example_arrayOfScalars показывает массив скаляров
// Ограничения элемента задаёт правило-значение в Items
func Example_arrayOfScalars() {
	printSchema[arrayScalarSpec]()
	// Output:
	// properties:
	//   uris:
	//     items:
	//       format: uri
	//       pattern: ^https://
	//       type: string
	//     minItems: 1
	//     type: array
	// required:
	// - uris
	// type: object
}

// arrayJWK это элемент массива со своим описанием
type arrayJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
}

func (s *arrayJWK) CRD(b crd.Builder) error {
	b.String(&s.Kty).Enum("RSA")
	b.String(&s.Kid).MaxLength(64)
	return nil
}

// arrayObjectSpec это массив структур
type arrayObjectSpec struct {
	Keys []arrayJWK `json:"keys"`
}

func (s *arrayObjectSpec) CRD(b crd.Builder) error {
	b.Array(&s.Keys).MinItems(1).MaxItems(5).ListType("map").ListMapKeys("kid")
	return nil
}

// Example_arrayOfObjects показывает массив структур
// Ограничения массива заданы на поле, элемент описан на своём типе
func Example_arrayOfObjects() {
	printSchema[arrayObjectSpec]()
	// Output:
	// properties:
	//   keys:
	//     items:
	//       properties:
	//         kid:
	//           maxLength: 64
	//           type: string
	//         kty:
	//           enum:
	//           - RSA
	//           type: string
	//       required:
	//       - kid
	//       - kty
	//       type: object
	//     maxItems: 5
	//     minItems: 1
	//     type: array
	//     x-kubernetes-list-map-keys:
	//     - kid
	//     x-kubernetes-list-type: map
	// required:
	// - keys
	// type: object
}

// rawSpec это поле с произвольным JSON
type rawSpec struct {
	Config any `json:"config"`
}

func (s *rawSpec) CRD(b crd.Builder) error {
	b.Raw(&s.Config).Description("произвольный JSON")
	return nil
}

// Example_raw показывает поле с произвольным JSON без схемы содержимого
func Example_raw() {
	printSchema[rawSpec]()
	// Output:
	// properties:
	//   config:
	//     description: произвольный JSON
	//     x-kubernetes-preserve-unknown-fields: true
	// required:
	// - config
	// type: object
}

// pointerAddress это адрес для полей по указателю и встраивания
type pointerAddress struct {
	City string `json:"city"`
	Zip  string `json:"zip,omitempty"`
}

// pointerMeta встраивается без тега, его поля вливаются в родителя
type pointerMeta struct {
	Owner string `json:"owner"`
}

// pointerSpec показывает, что адресуется указателем
type pointerSpec struct {
	pointerMeta
	Home   pointerAddress  `json:"home"`
	Backup *pointerAddress `json:"backup,omitempty"`
}

func (s *pointerSpec) CRD(b crd.Builder) error {
	b.String(&s.Owner).MinLength(1)
	b.String(&s.Home.City).Pattern("^[A-Z]").Optional()
	b.String(&s.Backup.Zip).Required()
	return nil
}

// Example_fieldPointers показывает, какие поля адресуются указателем
// Поле встроенной структуры, поле вложенной структуры и поле структуры за указателем.
// Optional для home.city не затрагивает backup.city того же типа
func Example_fieldPointers() {
	printSchema[pointerSpec]()
	// Output:
	// properties:
	//   backup:
	//     properties:
	//       city:
	//         type: string
	//       zip:
	//         type: string
	//     required:
	//     - city
	//     - zip
	//     type: object
	//   home:
	//     properties:
	//       city:
	//         pattern: ^[A-Z]
	//         type: string
	//       zip:
	//         type: string
	//     type: object
	//   owner:
	//     minLength: 1
	//     type: string
	// required:
	// - home
	// - owner
	// type: object
}
