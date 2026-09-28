package crd

import (
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"crd.tools/crd/formats"
)

type address struct {
	City string `json:"city"`
	Zip  string `json:"zip,omitempty"`
}

type common struct {
	Owner string `json:"owner"`
}

// userClientID описывает свою схему как тип-значение
type userClientID string

func (userClientID) Rule() (Rule, error) {
	return K8sName().MaxLength(64), nil
}

// snap берётся из снимков по метке
type snap struct {
	V string `json:"v"`
}

type userSpec struct {
	common
	ClientID userClientID      `json:"clientId"`
	Name     string            `json:"name"`
	Mode     string            `json:"mode"`
	TTL      int               `json:"ttl,omitempty"`
	Keys     []string          `json:"keys,omitzero"`
	Labels   map[string]string `json:"labels,omitempty"`
	Address  address           `json:"address"`
	Backup   *address          `json:"backup,omitempty"`
	When     metav1.Time       `json:"when"`
	CPU      resource.Quantity `json:"cpu"`
	Legacy   []snap            `json:"legacy" crd:"kubebuild"`
	Skipped  string            `json:"-"`
}

func (s *userSpec) CRD(b Builder) error {
	b.Self().Description("user")
	b.String(&s.ClientID).Description("client")
	b.String(&s.Name).MaxLength(128)
	b.String(&s.Mode).OneOf().Enum("fast")
	b.String(&s.Mode).OneOf().Pattern("^slow")
	b.DurationSeconds(&s.TTL, 60, 3600).Default(900).Required()
	b.Array(&s.Keys).MinItems(1).Items(HTTPSURL())
	b.Map(&s.Labels).Values(NonEmptyString())
	b.String(&s.Address.City).Pattern("^[A-Z]").Optional()
	b.String(&s.Backup.Zip).Required()
	b.String(&s.Owner).MinLength(1)
	b.Object(&s.Address).XValidation("self.city != ''", "city required")
	return nil
}

type badKinds struct {
	N int    `json:"n"`
	S string `json:"s"`
}

func (s *badKinds) CRD(b Builder) error {
	var local string
	b.String(&s.N)
	b.String(&local)
	b.Integer(&s.S).Required()
	return nil
}

type opaque struct{ V string }

func (opaque) MarshalJSON() ([]byte, error) { return nil, nil }

type withOpaque struct {
	O opaque `json:"o"`
}

func (*withOpaque) CRD(Builder) error { return nil }

type loop struct {
	Next *loop `json:"next,omitempty"`
}

func (*loop) CRD(Builder) error { return nil }

// structural проверяет схему официальным валидатором структурных схем
func structural(s *apiextv1.JSONSchemaProps) error {
	var internal apiext.JSONSchemaProps
	if err := apiextv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(s, &internal, nil); err != nil {
		return err
	}
	ss, err := structuralschema.NewStructural(&internal)
	if err != nil {
		return err
	}
	return structuralschema.ValidateStructural(nil, ss).ToAggregate()
}

func registryKey[T any]() string {
	t := reflect.TypeFor[T]()
	return t.PkgPath() + "." + t.Name()
}

func TestBuilder(t *testing.T) {
	Convey("Describer builds a schema by field pointers", t, func() {
		st := newState()
		So(st.registerSchema(snap{}, apiextv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]apiextv1.JSONSchemaProps{"v": {Type: "string"}},
		}), ShouldBeNil)

		s, err := st.schemaFor(reflect.TypeFor[userSpec]())
		So(err, ShouldBeNil)
		So(structural(s), ShouldBeNil)

		So(s.Description, ShouldEqual, "user")
		So(s.Properties, ShouldContainKey, "owner")
		So(s.Properties, ShouldNotContainKey, "common")
		So(s.Properties, ShouldNotContainKey, "Skipped")
		So(s.Required, ShouldResemble, []string{
			"address", "clientId", "cpu", "legacy", "mode", "name", "owner", "ttl", "when",
		})

		clientID := s.Properties["clientId"]
		So(clientID.Pattern, ShouldEqual, k8sNamePattern)
		So(*clientID.MaxLength, ShouldEqual, 64)
		So(clientID.Description, ShouldEqual, "client")

		So(s.Properties["mode"].OneOf, ShouldHaveLength, 2)
		So(s.Properties["mode"].OneOf[0].Type, ShouldBeEmpty)

		ttl := s.Properties["ttl"]
		So(*ttl.Minimum, ShouldEqual, 60)
		So(string(ttl.Default.Raw), ShouldEqual, "900")

		So(s.Properties["keys"].Items.Schema.Pattern, ShouldEqual, httpsPattern)
		So(*s.Properties["labels"].AdditionalProperties.Schema.MinLength, ShouldEqual, 1)

		addr := s.Properties["address"]
		So(addr.Properties["city"].Pattern, ShouldEqual, "^[A-Z]")
		So(addr.Required, ShouldBeEmpty)
		So(addr.XValidations, ShouldHaveLength, 1)
		// Optional для address.city не протекает в backup.city того же типа
		So(s.Properties["backup"].Required, ShouldResemble, []string{"city", "zip"})

		So(s.Properties["when"].Format, ShouldEqual, "date-time")
		So(s.Properties["cpu"].XIntOrString, ShouldBeTrue)
		So(s.Properties["legacy"].Items.Schema.Properties, ShouldContainKey, "v")

		Convey("field types with Ruler are registered and referenced by $ref", func() {
			raw, err := st.reg.Get(registryKey[userSpec]())
			So(err, ShouldBeNil)
			So(*raw.Properties["clientId"].Ref, ShouldEqual, registryKey[userClientID]())
			So(st.reg.HasRuntime(registryKey[userClientID]()), ShouldBeTrue)
		})
	})

	Convey("errors point to the line in CRD", t, func() {
		_, err := newState().schemaFor(reflect.TypeFor[badKinds]())
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "builder_test.go:")
		So(err.Error(), ShouldContainSubstring, "string rule is not applicable to int")
		So(err.Error(), ShouldContainSubstring, "is not a pointer to a field of badKinds")
		So(err.Error(), ShouldContainSubstring, "integer rule is not applicable to string")
		So(Code(err), ShouldEqual, RuleKindMismatch)
	})

	Convey("types with custom JSON need a rule", t, func() {
		_, err := newState().schemaFor(reflect.TypeFor[withOpaque]())
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "own JSON encoding")
		So(Code(err), ShouldEqual, UnsupportedType)
	})

	Convey("recursive types are rejected", t, func() {
		_, err := newState().schemaFor(reflect.TypeFor[loop]())
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "recursive")
		So(Code(err), ShouldEqual, RecursiveType)
	})

	Convey("value rules", t, func() {
		// Required у правила-значения нет: NewString().Required() не компилируется
		s, err := NewArray(Port()).MaxItems(3).Schema()
		So(err, ShouldBeNil)
		So(*s.Items.Schema.Maximum, ShouldEqual, 65535)

		_, err = NewArray(nil).Schema()
		So(Code(err), ShouldEqual, InvalidRule)

		f, err := NewString().Format(formats.DateTime).Schema()
		So(err, ShouldBeNil)
		So(f.Format, ShouldEqual, "date-time")
	})
}
