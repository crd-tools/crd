package crd

import (
	"reflect"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"crd.tools/crd/registry"
)

// oaAddress обычная структура без описания, должна стать компонентом
type oaAddress struct {
	City string `json:"city"`
	Zip  string `json:"zip,omitempty"`
}

// oaID описывает себя через Rule
type oaID string

func (oaID) Rule() (Rule, error) { return K8sName().MaxLength(64), nil }

// oaTag элемент для Of
type oaTag struct {
	Name string `json:"name"`
}

type oaSpec struct {
	Owner   oaID                 `json:"owner"`
	Home    oaAddress            `json:"home"`
	Work    oaAddress            `json:"work"`
	Changed oaAddress            `json:"changed"`
	Tags    []oaTag              `json:"tags"`
	Legacy  prioLegacy           `json:"legacy" crd:"kubebuild"`
	At      metav1.Time          `json:"at"`
	Sel     metav1.LabelSelector `json:"sel"`
}

func (s *oaSpec) CRD(b Builder) error {
	b.String(&s.Owner).Description("владелец")
	b.Object(&s.Work).Description("рабочий адрес")         // только своё свойство — ссылка через allOf
	b.String(&s.Changed.City).Pattern("^[A-Z]").Optional() // меняет поля — описание на месте
	b.Array(&s.Tags).Items(Of[oaTag]())
	return nil
}

type oaResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec oaSpec `json:"spec"`
}

// thisPkg это имя компонента для текущего пакета без префикса
const thisPkg = "crd.tools.crd"

func oaDoc(t *testing.T, prefixes ...Prefix) (*OpenAPIDocument, error) {
	st := newStateWithStd(t)
	st.setDERSource(snapshotOf(t, prioSnapshot(t)))
	c := newResource(st, reflect.TypeFor[oaResource]()).
		Group("example.com").Kind("Thing").Plural("things").Version("v1", true, true)
	return st.openAPI(OpenAPIConfig{Title: "t", Version: "v1", Prefixes: prefixes}, []CRD{c})
}

// refsResolve проверяет, что каждая ссылка ведёт на существующий компонент
func refsResolve(doc *OpenAPIDocument) []string {
	var broken []string
	for name, s := range doc.Components.Schemas {
		s := s
		registry.Walk(&s, func(n *apiextv1.JSONSchemaProps) bool {
			if n.Ref != nil {
				target := strings.TrimPrefix(*n.Ref, componentsPrefix)
				if _, ok := doc.Components.Schemas[target]; !ok || !strings.HasPrefix(*n.Ref, componentsPrefix) {
					broken = append(broken, name+" -> "+*n.Ref)
				}
			}
			return true
		})
	}
	return broken
}

func ref(name string) string { return componentsPrefix + name }

func TestOpenAPI(t *testing.T) {
	Convey("OpenAPI splits named types into components", t, func() {
		doc, err := oaDoc(t,
			Prefix{From: thisPkg, To: ""},
			Prefix{From: "k8s.io.apimachinery.pkg", To: "apimachinery"},
		)
		So(err, ShouldBeNil)
		So(doc.OpenAPI, ShouldEqual, OpenAPIVersion)
		So(doc.Paths, ShouldNotBeNil)
		So(refsResolve(doc), ShouldBeEmpty)

		c := doc.Components.Schemas
		So(c, ShouldContainKey, "oaResource")
		So(c, ShouldContainKey, "oaSpec")
		So(c, ShouldContainKey, "oaAddress")
		So(c, ShouldContainKey, "oaTag")
		So(c, ShouldContainKey, "oaID")
		So(c, ShouldContainKey, "apimachinery.apis.meta.v1.LabelSelector")
		So(c, ShouldContainKey, "apimachinery.apis.meta.v1.LabelSelectorRequirement")

		root, spec := c["oaResource"], c["oaSpec"]
		So(root.Properties, ShouldContainKey, "apiVersion")
		So(*root.Properties["spec"].Ref, ShouldEqual, ref("oaSpec"))

		Convey("a position without own rules is a plain $ref", func() {
			So(*spec.Properties["home"].Ref, ShouldEqual, ref("oaAddress"))
			So(*spec.Properties["sel"].Ref, ShouldEqual, ref("apimachinery.apis.meta.v1.LabelSelector"))
		})

		Convey("own properties of a position go next to allOf", func() {
			work := spec.Properties["work"]
			So(work.Ref, ShouldBeNil)
			So(work.Description, ShouldEqual, "рабочий адрес")
			So(*work.AllOf[0].Ref, ShouldEqual, ref("oaAddress"))
			So(work.Properties, ShouldBeEmpty)

			owner := spec.Properties["owner"]
			So(owner.Description, ShouldEqual, "владелец")
			So(*owner.AllOf[0].Ref, ShouldEqual, ref("oaID"))
			So(c["oaID"].Pattern, ShouldEqual, k8sNamePattern)
		})

		Convey("a position changed by the parent is described in place", func() {
			changed := spec.Properties["changed"]
			So(changed.Ref, ShouldBeNil)
			So(changed.AllOf, ShouldBeEmpty)
			So(changed.Properties["city"].Pattern, ShouldEqual, "^[A-Z]")
			So(changed.Required, ShouldBeEmpty)
			So(c["oaAddress"].Required, ShouldResemble, []string{"city"}) // общий компонент не тронут
		})

		Convey("Of in Items refers to the component", func() {
			So(*spec.Properties["tags"].Items.Schema.Ref, ShouldEqual, ref("oaTag"))
		})

		Convey("a tagged field refers to the snapshot, nested refs are converted", func() {
			So(*spec.Properties["legacy"].Ref, ShouldEqual, ref("prioLegacy"))
			So(*c["prioLegacy"].Properties["inner"].Ref, ShouldEqual, ref("prioInner"))
			So(*c["prioInner"].Properties["v"].MaxLength, ShouldEqual, 10)
		})

		Convey("known types stay inline", func() {
			So(spec.Properties["at"].Format, ShouldEqual, "date-time")
			So(c, ShouldNotContainKey, "apimachinery.apis.meta.v1.Time")
		})
	})

	Convey("component names", t, func() {
		So(componentName("k8s.io/apimachinery/pkg/apis/meta/v1.Time", nil),
			ShouldEqual, "k8s.io.apimachinery.pkg.apis.meta.v1.Time")
		So(componentName("k8s.io/apimachinery/pkg/apis/meta/v1.Time", []Prefix{
			{From: "k8s.io", To: "k8s"},
			{From: "k8s.io.apimachinery.pkg", To: "apimachinery"},
		}), ShouldEqual, "apimachinery.apis.meta.v1.Time") // самый длинный префикс
		So(componentName("k8s.io/apimachinery/pkg/apis/meta/v1.Time", []Prefix{
			{From: "k8s.io.api", To: "api"},
		}), ShouldEqual, "k8s.io.apimachinery.pkg.apis.meta.v1.Time") // только по границе точки
		So(componentName("example.com/a+b/v1.T", nil), ShouldEqual, "example.com.a_b.v1.T")
	})

	Convey("same names after prefixes are an error", t, func() {
		_, err := oaDoc(t,
			Prefix{From: thisPkg, To: "x"},
			Prefix{From: "k8s.io.apimachinery.pkg.apis.meta.v1", To: "x"},
		)
		// LabelSelector есть только в apimachinery, совпадения нет
		So(err, ShouldBeNil)

		g := &openAPIGen{names: map[string]string{}, keys: map[string]string{},
			cfg: OpenAPIConfig{Prefixes: []Prefix{{From: "a", To: "x"}, {From: "b", To: "x"}}}}
		_, err = g.name("a.T")
		So(err, ShouldBeNil)
		_, err = g.name("b.T")
		So(Code(err), ShouldEqual, InvalidArgument)
		So(err.Error(), ShouldContainSubstring, "adjust the prefixes")
	})

	Convey("definitions not made by crd.New are rejected", t, func() {
		_, err := newState().openAPI(OpenAPIConfig{}, []CRD{nil})
		So(Code(err), ShouldEqual, InvalidArgument)
	})
}
