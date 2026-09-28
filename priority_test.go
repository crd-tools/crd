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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Тесты фиксируют порядок, описанный в README в разделе «Откуда берётся схема»

// expectation это ожидаемые ключевые свойства схемы
type expectation struct {
	typ      string
	format   string
	intOrStr bool
	preserve bool
	pattern  string
}

func (e expectation) check(s apiextv1.JSONSchemaProps) {
	So(s.Type, ShouldEqual, e.typ)
	So(s.Format, ShouldEqual, e.format)
	So(s.XIntOrString, ShouldEqual, e.intOrStr)
	So(s.XPreserveUnknownFields != nil && *s.XPreserveUnknownFields, ShouldEqual, e.preserve)
	So(s.Pattern, ShouldEqual, e.pattern)
}

// knownCases это все типы из таблицы известных типов
var knownCases = []struct {
	name string
	t    reflect.Type
	want expectation
}{
	{"time.Time", reflect.TypeFor[time.Time](), expectation{typ: "string", format: "date-time"}},
	{"json.RawMessage", reflect.TypeFor[json.RawMessage](), expectation{preserve: true}},
	{"metav1.Time", reflect.TypeFor[metav1.Time](), expectation{typ: "string", format: "date-time"}},
	{"metav1.MicroTime", reflect.TypeFor[metav1.MicroTime](), expectation{typ: "string", format: "date-time"}},
	{"metav1.Duration", reflect.TypeFor[metav1.Duration](), expectation{typ: "string"}},
	{"metav1.ObjectMeta", reflect.TypeFor[metav1.ObjectMeta](), expectation{typ: "object"}},
	{"resource.Quantity", reflect.TypeFor[resource.Quantity](), expectation{intOrStr: true, pattern: quantityPattern}},
	{"intstr.IntOrString", reflect.TypeFor[intstr.IntOrString](), expectation{intOrStr: true}},
	{"runtime.RawExtension", reflect.TypeFor[runtime.RawExtension](), expectation{typ: "object", preserve: true}},
	{"apiextv1.JSON", reflect.TypeFor[apiextv1.JSON](), expectation{preserve: true}},
}

// knownFields содержит все известные типы полями, в том числе в обёртках
type knownFields struct {
	Time       time.Time            `json:"time"`
	Raw        json.RawMessage      `json:"raw"`
	MetaTime   metav1.Time          `json:"metaTime"`
	MicroTime  metav1.MicroTime     `json:"microTime"`
	Duration   metav1.Duration      `json:"duration"`
	ObjectMeta metav1.ObjectMeta    `json:"objectMeta"`
	Quantity   resource.Quantity    `json:"quantity"`
	IntOrStr   intstr.IntOrString   `json:"intOrStr"`
	Extension  runtime.RawExtension `json:"extension"`
	JSON       apiextv1.JSON        `json:"json"`

	TimePtr    *metav1.Time                 `json:"timePtr,omitempty"`
	Times      []time.Time                  `json:"times"`
	Quantities map[string]resource.Quantity `json:"quantities"`
}

var knownFieldNames = map[string]string{
	"time.Time":            "time",
	"json.RawMessage":      "raw",
	"metav1.Time":          "metaTime",
	"metav1.MicroTime":     "microTime",
	"metav1.Duration":      "duration",
	"metav1.ObjectMeta":    "objectMeta",
	"resource.Quantity":    "quantity",
	"intstr.IntOrString":   "intOrStr",
	"runtime.RawExtension": "extension",
	"apiextv1.JSON":        "json",
}

// ── типы для правил старшинства ──

// prioClientID описывает себя через Rule
type prioClientID string

func (prioClientID) Rule() (Rule, error) { return K8sName().MaxLength(64), nil }

// prioIP кодируется сам и описывает схему методами kube-openapi
type prioIP struct{ b [4]byte }

func (prioIP) MarshalJSON() ([]byte, error) { return nil, nil }
func (prioIP) OpenAPISchemaType() []string  { return []string{"string"} }
func (prioIP) OpenAPISchemaFormat() string  { return "ipv4" }

// prioOpaqueJSON кодируется сам и ничего о схеме не сообщает
type prioOpaqueJSON struct{ v string }

func (prioOpaqueJSON) MarshalJSON() ([]byte, error) { return nil, nil }

// prioOpaqueText кодируется сам через MarshalText
type prioOpaqueText struct{ v string }

func (prioOpaqueText) MarshalText() ([]byte, error) { return nil, nil }

// prioPlain без описания, выводится из Go-типа
type prioPlain struct {
	N int `json:"n"`
}

// prioLegacy попадает в схему по метке из снимка
type prioLegacy struct {
	Inner prioInner `json:"inner"`
}

// prioInner вложен в prioLegacy и в снимке, и в коде
type prioInner struct {
	V string `json:"v"`
}

type prioSpec struct {
	Owner  prioClientID `json:"owner"`
	Addr   prioIP       `json:"addr"`
	Plain  prioPlain    `json:"plain"`
	Legacy prioLegacy   `json:"legacy" crd:"kubebuild"`
}

func (s *prioSpec) CRD(b Builder) error {
	b.String(&s.Owner).Description("владелец")
	b.String(&s.Addr).Description("адрес")
	return nil
}

// prioWrapped встраивает тип из снимка целиком
type prioWrapped struct {
	prioLegacy `json:",inline" crd:"kubebuild"`

	Own string `json:"own"`
}

func (*prioWrapped) CRD(Builder) error { return nil }

type prioOpaqueJSONSpec struct {
	F prioOpaqueJSON `json:"f"`
}

func (*prioOpaqueJSONSpec) CRD(Builder) error { return nil }

type prioOpaqueTextSpec struct {
	F prioOpaqueText `json:"f"`
}

func (*prioOpaqueTextSpec) CRD(Builder) error { return nil }

type prioTimeField struct {
	At metav1.Time `json:"at"`
}

func (*prioTimeField) CRD(Builder) error { return nil }

// prioRuleOK правила на полях со своим JSON-представлением
type prioRuleOK struct {
	At   metav1.Time        `json:"at"`
	Port intstr.IntOrString `json:"port"`
}

func (s *prioRuleOK) CRD(b Builder) error {
	b.String(&s.At).Pattern("^20")
	b.String(&s.Port).MaxLength(15)
	b.Integer(&s.Port).Minimum(1)
	return nil
}

// prioRuleKinds проверяет вид правил по JSON-виду, а не по Go-виду
type prioRuleKinds struct {
	At    metav1.Time        `json:"at"`
	Port  intstr.IntOrString `json:"port"`
	Count metav1.Time        `json:"count"`
}

func (s *prioRuleKinds) CRD(b Builder) error {
	b.String(&s.At).Pattern("^20")  // metav1.Time в JSON — строка
	b.String(&s.Port).MaxLength(15) // число или строка принимает строковые правила
	b.Integer(&s.Port).Minimum(1)   // и целочисленные
	b.Integer(&s.Count).Minimum(0)  // а строка целочисленные — нет
	return nil
}

// prioSnapshot строит снимок prioLegacy с prioInner, у которого maxLength 10
func prioSnapshot(t *testing.T) map[string]apiextv1.JSONSchemaProps {
	ten := int64(10)
	inner := registryKey[prioInner]()
	return map[string]apiextv1.JSONSchemaProps{
		registryKey[prioLegacy](): {Type: "object", Properties: map[string]apiextv1.JSONSchemaProps{
			"inner": {Ref: &inner},
		}},
		inner: {Type: "object", Properties: map[string]apiextv1.JSONSchemaProps{
			"v": {Type: "string", MaxLength: &ten},
		}},
	}
}

func TestKnownTypes(t *testing.T) {
	Convey("every known type as a root", t, func() {
		for _, c := range knownCases {
			Convey(c.name, func() {
				s, err := newState().schemaFor(c.t)
				So(err, ShouldBeNil)
				c.want.check(*s)
			})
		}
	})

	Convey("every known type as a field", t, func() {
		s, err := newState().schemaFor(reflect.TypeFor[knownFields]())
		So(err, ShouldBeNil)
		So(structural(s), ShouldBeNil)
		for _, c := range knownCases {
			Convey(c.name, func() {
				c.want.check(s.Properties[knownFieldNames[c.name]])
			})
		}
	})

	Convey("known types behind a pointer, in a slice and in a map", t, func() {
		s, err := newState().schemaFor(reflect.TypeFor[knownFields]())
		So(err, ShouldBeNil)
		So(s.Properties["timePtr"].Format, ShouldEqual, "date-time")
		So(s.Properties["times"].Items.Schema.Format, ShouldEqual, "date-time")
		So(s.Properties["quantities"].AdditionalProperties.Schema.XIntOrString, ShouldBeTrue)
	})

	Convey("time.Duration has no own JSON and is an integer", t, func() {
		s, err := newState().schemaFor(reflect.TypeFor[builtinSpec]())
		So(err, ShouldBeNil)
		So(s.Properties["timeout"].Type, ShouldEqual, "integer")
	})
}

func TestPriorities(t *testing.T) {
	Convey("field rules overlay the base schema of the type", t, func() {
		st := newState()
		st.setDERSource(snapshotOf(t, prioSnapshot(t)))
		s, err := st.schemaFor(reflect.TypeFor[prioSpec]())
		So(err, ShouldBeNil)

		owner := s.Properties["owner"]
		So(owner.Pattern, ShouldEqual, k8sNamePattern) // из Rule типа
		So(*owner.MaxLength, ShouldEqual, 64)
		So(owner.Description, ShouldEqual, "владелец") // из правила поля
	})

	Convey("rule kinds are checked against the JSON kind, not the Go kind", t, func() {
		_, err := newState().schemaFor(reflect.TypeFor[prioRuleKinds]())
		So(Code(err), ShouldEqual, RuleKindMismatch)
		So(err.Error(), ShouldContainSubstring, "prioRuleKinds.Count: integer rule is not applicable")
		So(err.Error(), ShouldNotContainSubstring, "prioRuleKinds.At")
		So(err.Error(), ShouldNotContainSubstring, "prioRuleKinds.Port")

		s, err := newState().schemaFor(reflect.TypeFor[prioRuleOK]())
		So(err, ShouldBeNil)
		So(structural(s), ShouldBeNil)
		at, port := s.Properties["at"], s.Properties["port"]
		So(at.Format, ShouldEqual, "date-time")
		So(at.Pattern, ShouldEqual, "^20")
		So(port.XIntOrString, ShouldBeTrue)
		So(*port.MaxLength, ShouldEqual, 15)
		So(*port.Minimum, ShouldEqual, 1)
	})

	Convey("2 > 3: code beats the known types table", t, func() {
		st := newState()
		So(st.registerSchema(metav1.Time{}, apiextv1.JSONSchemaProps{Type: "string", Pattern: "^20"}), ShouldBeNil)
		s, err := st.schemaFor(reflect.TypeFor[prioTimeField]())
		So(err, ShouldBeNil)
		So(s.Properties["at"].Pattern, ShouldEqual, "^20")
		So(s.Properties["at"].Format, ShouldBeEmpty)
	})

	Convey("3 > 4: the table beats kube-openapi methods", t, func() {
		// resource.Quantity описывает себя методами как строку или число, таблица добавляет шаблон
		s, err := newState().schemaFor(reflect.TypeFor[resource.Quantity]())
		So(err, ShouldBeNil)
		So(s.Pattern, ShouldEqual, quantityPattern)
	})

	Convey("4 > 5: kube-openapi methods beat MarshalJSON", t, func() {
		st := newState()
		st.setDERSource(snapshotOf(t, prioSnapshot(t)))
		s, err := st.schemaFor(reflect.TypeFor[prioSpec]())
		So(err, ShouldBeNil)
		So(s.Properties["addr"].Format, ShouldEqual, "ipv4")
		So(s.Properties["addr"].Description, ShouldEqual, "адрес")
	})

	Convey("5: own JSON without a description is an error", t, func() {
		_, err := newState().schemaFor(reflect.TypeFor[prioOpaqueJSONSpec]())
		So(Code(err), ShouldEqual, UnsupportedType)
		_, err = newState().schemaFor(reflect.TypeFor[prioOpaqueTextSpec]())
		So(Code(err), ShouldEqual, UnsupportedType)
	})

	Convey("6: a plain type is derived from its Go type", t, func() {
		st := newState()
		st.setDERSource(snapshotOf(t, prioSnapshot(t)))
		s, err := st.schemaFor(reflect.TypeFor[prioSpec]())
		So(err, ShouldBeNil)
		So(s.Properties["plain"].Properties["n"].Type, ShouldEqual, "integer")
	})

	Convey("1: a tagged field comes from the snapshot", t, func() {
		st := newState()
		st.setDERSource(snapshotOf(t, prioSnapshot(t)))
		s, err := st.schemaFor(reflect.TypeFor[prioSpec]())
		So(err, ShouldBeNil)
		So(*s.Properties["legacy"].Properties["inner"].Properties["v"].MaxLength, ShouldEqual, 10)

		Convey("but code beats the snapshot, also inside it", func() {
			twenty := int64(20)
			So(st.registerSchema(prioInner{}, apiextv1.JSONSchemaProps{Type: "object", Properties: map[string]apiextv1.JSONSchemaProps{
				"v": {Type: "string", MaxLength: &twenty},
			}}), ShouldBeNil)
			s, err := st.schemaFor(reflect.TypeFor[prioSpec]())
			So(err, ShouldBeNil)
			So(*s.Properties["legacy"].Properties["inner"].Properties["v"].MaxLength, ShouldEqual, 20)
		})
	})

	Convey("an inline field with the tag merges the snapshot into the object", t, func() {
		st := newState()
		st.setDERSource(snapshotOf(t, prioSnapshot(t)))
		s, err := st.schemaFor(reflect.TypeFor[prioWrapped]())
		So(err, ShouldBeNil)
		So(s.AllOf, ShouldBeEmpty)
		So(s.Properties, ShouldContainKey, "own")
		So(*s.Properties["inner"].Properties["v"].MaxLength, ShouldEqual, 10) // из снимка, а не из Go-полей
		So(s.Required, ShouldContain, "own")
	})

	Convey("SchemaFor, New and Of for the type itself: code, snapshot, built-in, Go type", t, func() {
		timeKey := registryKey[time.Time]()

		Convey("snapshot beats the built-in schema for a root type", func() {
			st := newState()
			st.setDERSource(snapshotOf(t, map[string]apiextv1.JSONSchemaProps{
				timeKey: {Type: "integer"},
			}))
			s, err := st.schemaFor(reflect.TypeFor[time.Time]())
			So(err, ShouldBeNil)
			So(s.Type, ShouldEqual, "integer")
		})

		Convey("code beats the snapshot for a root type", func() {
			st := newState()
			st.setDERSource(snapshotOf(t, map[string]apiextv1.JSONSchemaProps{
				timeKey: {Type: "integer"},
			}))
			So(st.registerSchema(time.Time{}, apiextv1.JSONSchemaProps{Type: "string", Pattern: "^x"}), ShouldBeNil)
			s, err := st.schemaFor(reflect.TypeFor[time.Time]())
			So(err, ShouldBeNil)
			So(s.Pattern, ShouldEqual, "^x")
		})

		Convey("Of uses the same order", func() {
			newStateWithStd(t)
			arr, err := NewArray(Of[metav1.Time]()).Schema()
			So(err, ShouldBeNil)
			So(*arr.Items.Schema.Ref, ShouldEqual, registryKey[metav1.Time]())
			s, err := std.schema(registryKey[metav1.Time]())
			So(err, ShouldBeNil)
			So(s.Format, ShouldEqual, "date-time")
		})

		Convey("New uses the same order", func() {
			c := newResource(newState(), reflect.TypeFor[prioPlain]())
			obj, err := c.Group("e.com").Kind("P").Plural("ps").Version("v1", true, true).Build()
			So(err, ShouldBeNil)
			So(obj.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["n"].Type, ShouldEqual, "integer")
		})
	})
}
