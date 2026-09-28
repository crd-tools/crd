package crd

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ipAddr кодируется в JSON сам и описывает схему методами kube-openapi
type ipAddr struct{ b [4]byte }

func (ipAddr) MarshalJSON() ([]byte, error) { return nil, nil }
func (ipAddr) OpenAPISchemaType() []string  { return []string{"string"} }
func (ipAddr) OpenAPISchemaFormat() string  { return "ipv4" }

// portOrName может быть числом или строкой
type portOrName struct{ v string }

func (portOrName) MarshalJSON() ([]byte, error)  { return nil, nil }
func (portOrName) OpenAPISchemaType() []string   { return []string{"string"} }
func (portOrName) OpenAPIV3OneOfTypes() []string { return []string{"integer", "string"} }

// flagOrText нельзя выразить схемой CRD
type flagOrText struct{}

func (flagOrText) OpenAPISchemaType() []string { return []string{"string", "boolean"} }

type builtinSpec struct {
	Created  time.Time         `json:"created"`
	Timeout  time.Duration     `json:"timeout"`
	Raw      json.RawMessage   `json:"raw"`
	Addr     ipAddr            `json:"addr"`
	Port     portOrName        `json:"port"`
	CPU      resource.Quantity `json:"cpu"`
	Deadline metav1.Duration   `json:"deadline"`
}

func (*builtinSpec) CRD(Builder) error { return nil }

type badOpenAPI struct {
	F flagOrText `json:"f"`
}

func (*badOpenAPI) CRD(Builder) error { return nil }

type overrideTime struct {
	At metav1.Time `json:"at"`
}

func (*overrideTime) CRD(Builder) error { return nil }

func TestBuiltin(t *testing.T) {
	Convey("types with their own JSON representation", t, func() {
		s, err := newState().schemaFor(reflect.TypeFor[builtinSpec]())
		So(err, ShouldBeNil)
		So(structural(s), ShouldBeNil)

		p := s.Properties
		So(p["created"].Type, ShouldEqual, "string")
		So(p["created"].Format, ShouldEqual, "date-time")
		So(p["timeout"].Type, ShouldEqual, "integer") // time.Duration — число наносекунд
		So(*p["raw"].XPreserveUnknownFields, ShouldBeTrue)
		So(p["addr"].Format, ShouldEqual, "ipv4") // методы kube-openapi важнее MarshalJSON
		So(p["port"].XIntOrString, ShouldBeTrue)
		So(p["cpu"].Pattern, ShouldEqual, quantityPattern) // таблица важнее методов
		So(p["deadline"].Type, ShouldEqual, "string")
	})

	Convey("OpenAPI types that a CRD cannot express are an error", t, func() {
		_, err := newState().schemaFor(reflect.TypeFor[badOpenAPI]())
		So(Code(err), ShouldEqual, UnsupportedType)
	})

	Convey("RegisterSchema overrides the built-in schema", t, func() {
		st := newState()
		So(st.registerSchema(metav1.Time{}, apiextv1.JSONSchemaProps{Type: "string", Pattern: "^20"}), ShouldBeNil)
		s, err := st.schemaFor(reflect.TypeFor[overrideTime]())
		So(err, ShouldBeNil)
		So(s.Properties["at"].Pattern, ShouldEqual, "^20")
	})

	Convey("SchemaFor and Of use the built-in schema of a known root type", t, func() {
		s, err := newStateWithStd(t).schemaFor(reflect.TypeFor[metav1.Time]())
		So(err, ShouldBeNil)
		So(s.Format, ShouldEqual, "date-time")

		arr, err := NewArray(Of[metav1.Time]()).Schema()
		So(err, ShouldBeNil)
		So(*arr.Items.Schema.Ref, ShouldEqual, "k8s.io/apimachinery/pkg/apis/meta/v1.Time")
	})
}
