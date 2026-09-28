package crd

import (
	"reflect"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// modWrapped встраивает тип из снимка, модификаторы правят его схему
type modWrapped struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec prioLegacy `json:"spec" crd:"kubebuild"`
}

// setMaxLength ставит maxLength полю v во вложенном типе prioInner
func setMaxLength(n int64) Modifier {
	return ModifierFunc(func(key string, s *apiextv1.JSONSchemaProps) error {
		if key == registryKey[prioInner]() {
			p := s.Properties["v"]
			p.MaxLength = &n
			s.Properties["v"] = p
		}
		return nil
	})
}

func modResource(st *state) *resourceBuilder {
	r := newResource(st, reflect.TypeFor[modWrapped]())
	r.Group("e.com").Kind("Mod").Plural("mods").Version("v1", true, true)
	return r
}

func innerMaxLength(t *testing.T, r *resourceBuilder) int64 {
	obj, err := r.Build()
	if err != nil {
		t.Fatal(err)
	}
	return *obj.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["inner"].Properties["v"].MaxLength
}

func TestModifiers(t *testing.T) {
	Convey("modifiers of a definition are applied on every build independently", t, func() {
		st := newState()
		st.setDERSource(snapshotOf(t, prioSnapshot(t)))
		r := modResource(st)

		So(innerMaxLength(t, r), ShouldEqual, 10) // из снимка

		r.Modifiers(setMaxLength(20))
		So(innerMaxLength(t, r), ShouldEqual, 20)

		r.Modifiers(setMaxLength(30))
		So(innerMaxLength(t, r), ShouldEqual, 30)

		r.Modifiers()
		So(innerMaxLength(t, r), ShouldEqual, 10)

		Convey("SchemaFor and other definitions do not see them", func() {
			r.Modifiers(setMaxLength(40))
			So(innerMaxLength(t, r), ShouldEqual, 40)

			other := modResource(st)
			So(innerMaxLength(t, other), ShouldEqual, 10)

			s, err := st.schemaFor(reflect.TypeFor[modWrapped]())
			So(err, ShouldBeNil)
			So(*s.Properties["spec"].Properties["inner"].Properties["v"].MaxLength, ShouldEqual, 10)
		})
	})

	Convey("modifier errors carry ModifierFailed", t, func() {
		st := newState()
		st.setDERSource(snapshotOf(t, prioSnapshot(t)))
		r := modResource(st)
		r.Modifiers(ModifierFunc(func(string, *apiextv1.JSONSchemaProps) error {
			return newError(InvalidRule, "broken")
		}))
		_, err := r.Build()
		So(Code(err), ShouldEqual, ModifierFailed)
		So(strings.Contains(err.Error(), "broken"), ShouldBeTrue)

		So(Code(modResource(st).Modifiers(nil).Err()), ShouldEqual, InvalidArgument)
	})

	Convey("OpenAPI builds components with the modifiers of each definition", t, func() {
		st := newStateWithStd(t)
		st.setDERSource(snapshotOf(t, prioSnapshot(t)))

		a := modResource(st)
		a.Modifiers(setMaxLength(20))
		doc, err := st.openAPI(OpenAPIConfig{}, []CRD{a})
		So(err, ShouldBeNil)
		So(*doc.Components.Schemas[thisPkg+".prioInner"].Properties["v"].MaxLength, ShouldEqual, 20)

		Convey("same components with different modifiers are an error", func() {
			b := newResource(st, reflect.TypeFor[modWrapped]())
			b.Group("e.com").Kind("Other").Plural("others").Version("v1", true, true)
			_, err := st.openAPI(OpenAPIConfig{}, []CRD{a, b})
			So(Code(err), ShouldEqual, InvalidArgument)
			So(err.Error(), ShouldContainSubstring, "use the same modifiers")

			b.Modifiers(setMaxLength(20))
			_, err = st.openAPI(OpenAPIConfig{}, []CRD{a, b})
			So(err, ShouldBeNil)
		})
	})
}
