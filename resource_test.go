package crd

import (
	"bytes"
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type widgetSpec struct {
	Replicas int32  `json:"replicas"`
	Mode     string `json:"mode,omitempty"`
}

func (s *widgetSpec) CRD(b Builder) error {
	b.Integer(&s.Replicas).Minimum(1).Maximum(10)
	b.String(&s.Mode).Enum("fast", "safe").Default("safe")
	return nil
}

type widget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec widgetSpec `json:"spec"`
}

// plainThing описывается выводом из Go-типа, без Describer
type plainThing struct {
	metav1.TypeMeta `json:",inline"`
	Size            int `json:"size"`
}

func TestResource(t *testing.T) {
	Convey("New builds a CRD from the type schema", t, func() {
		st := newState()
		c := newResource(st, reflect.TypeFor[widget]())
		c.Group("example.com").Kind("Widget").Plural("widgets").Version("v1", true, true)

		obj, err := c.Build()
		So(err, ShouldBeNil)
		So(structural(obj.Spec.Versions[0].Schema.OpenAPIV3Schema), ShouldBeNil)
		So(obj.Spec.Names.ListKind, ShouldEqual, "WidgetList")
		So(obj.Name, ShouldEqual, "widgets.example.com")
		So(obj.Spec.Names.Singular, ShouldEqual, "widget")
		So(obj.Spec.Scope, ShouldEqual, apiextv1.NamespaceScoped)

		root := obj.Spec.Versions[0].Schema.OpenAPIV3Schema
		So(root.Properties, ShouldContainKey, "apiVersion")
		So(root.Properties["metadata"].Type, ShouldEqual, "object")
		So(root.Required, ShouldResemble, []string{"spec"})
		spec := root.Properties["spec"]
		So(*spec.Properties["replicas"].Maximum, ShouldEqual, 10)
		So(spec.Properties["mode"].Enum, ShouldHaveLength, 2)

		So(c.GroupVersionKind("v1").String(), ShouldEqual, "example.com/v1, Kind=Widget")
		So(c.GroupVersionResource("v1").Resource, ShouldEqual, "widgets")
		So(c.Namespaced(), ShouldBeTrue)
		So(c.HasStatus("v1"), ShouldBeFalse)

		var buf bytes.Buffer
		So(c.Encode(&buf), ShouldBeNil)
		So(buf.String(), ShouldStartWith, "---\n")
		So(buf.String(), ShouldContainSubstring, "kind: CustomResourceDefinition")
		So(buf.String(), ShouldNotContainSubstring, "status:")
	})

	Convey("a plain struct is described from its Go type", t, func() {
		c := newResource(newState(), reflect.TypeFor[plainThing]())
		obj, err := c.Group("e.com").Kind("Thing").Plural("things").Version("v1", true, true).Build()
		So(err, ShouldBeNil)
		So(structural(obj.Spec.Versions[0].Schema.OpenAPIV3Schema), ShouldBeNil)
		So(obj.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["size"].Type, ShouldEqual, "integer")
	})

	Convey("errors carry codes", t, func() {
		_, err := newResource(newState(), reflect.TypeFor[int]()).Build()
		So(Code(err), ShouldEqual, NotStruct)

		_, err = newResource(newState(), reflect.TypeFor[widget]()).Kind("W").Plural("ws").Version("v1", true, true).Build()
		So(Code(err), ShouldEqual, InvalidArgument)

		_, err = newResource(newState(), reflect.TypeFor[widget]()).
			Group("e.com").Kind("W").Plural("ws").Version("v1", true, false).Build()
		So(Code(err), ShouldEqual, InvalidArgument)

		_, err = newResource(newState(), reflect.TypeFor[widget]()).
			Group("e.com").Kind("W").Plural("ws").
			Version("v1", true, true, PrinterColumn("X", "float", ".spec.x")).Build()
		So(Code(err), ShouldEqual, InvalidArgument)

		_, err = newResource(newState(), reflect.TypeFor[widget]()).
			Group("e.com").Kind("W").Plural("ws").
			Version("v1", true, true, VersionType[int]()).Build()
		So(Code(err), ShouldEqual, NotStruct)
	})
}
