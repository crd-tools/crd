package crd

import (
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// inlineForeign это чужой ресурс со схемой в снимке, Go-поля схему не задают
type inlineForeign struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec inlineForeignSpec `json:"spec"`
}

type inlineForeignSpec struct {
	Replicas int32 `json:"replicas"`
}

// inlineWrapper подключает чужой ресурс целиком
type inlineWrapper struct {
	inlineForeign `json:",inline" crd:"kubebuild"`
}

func inlineSnapshot(t *testing.T) map[string]apiextv1.JSONSchemaProps {
	five := 5.0
	spec := registryKey[inlineForeignSpec]()
	return map[string]apiextv1.JSONSchemaProps{
		registryKey[inlineForeign](): {
			Type:        "object",
			Description: "чужой ресурс",
			Properties: map[string]apiextv1.JSONSchemaProps{
				"apiVersion": {Type: "string"},
				"kind":       {Type: "string"},
				"metadata":   {Type: "object"},
				"spec":       {Ref: &spec},
			},
			Required: []string{"spec"},
		},
		spec: {
			Type:        "object",
			Description: "спецификация из маркеров",
			Properties: map[string]apiextv1.JSONSchemaProps{
				"replicas": {Type: "integer", Format: "int32", Maximum: &five},
			},
			Required: []string{"replicas"},
		},
	}
}

func inlineResource(t *testing.T) (*state, CRD) {
	st := newStateWithStd(t)
	st.setDERSource(snapshotOf(t, inlineSnapshot(t)))
	c := newResource(st, reflect.TypeFor[inlineWrapper]()).
		Group("example.com").Kind("Foreign").Plural("foreigns").Version("v1", true, true)
	return st, c
}

func TestInlineSnapshot(t *testing.T) {
	Convey("чужой ресурс, встроенный с меткой, берётся из снимка целиком", t, func() {
		_, c := inlineResource(t)
		obj, err := c.Build()
		So(err, ShouldBeNil)
		s := obj.Spec.Versions[0].Schema.OpenAPIV3Schema

		So(s.AllOf, ShouldBeEmpty)
		So(s.Description, ShouldEqual, "чужой ресурс")
		So(s.Properties, ShouldContainKey, "apiVersion")
		So(s.Properties, ShouldContainKey, "kind")
		So(s.Properties, ShouldContainKey, "metadata")
		So(s.Required, ShouldContain, "spec")

		spec := s.Properties["spec"]
		So(spec.Description, ShouldEqual, "спецификация из маркеров")
		So(*spec.Properties["replicas"].Maximum, ShouldEqual, 5)
		So(spec.Required, ShouldContain, "replicas")
		So(structural(s), ShouldBeNil)
	})

	Convey("в OpenAPI встроенный тип — отдельный компонент через allOf", t, func() {
		st, c := inlineResource(t)
		doc, err := st.openAPI(OpenAPIConfig{Title: "t", Version: "v1"}, []CRD{c})
		So(err, ShouldBeNil)
		So(refsResolve(doc), ShouldBeEmpty)

		wrapper := doc.Components.Schemas[thisPkg+".inlineWrapper"]
		So(wrapper.AllOf, ShouldHaveLength, 1)
		So(*wrapper.AllOf[0].Ref, ShouldEqual, "#/components/schemas/"+thisPkg+".inlineForeign")

		foreign := doc.Components.Schemas[thisPkg+".inlineForeign"]
		So(foreign.Properties, ShouldContainKey, "spec")
	})
}
