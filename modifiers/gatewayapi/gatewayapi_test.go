package gatewayapi

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// schema это тип, в описаниях которого есть теги Gateway API
func schema() apiextv1.JSONSchemaProps {
	return apiextv1.JSONSchemaProps{
		Type:        "object",
		Description: "Type doc.\n\n<gateway:util:excludeFromCRD>internal notes</gateway:util:excludeFromCRD>",
		Required:    []string{"stable", "exp"},
		Properties: map[string]apiextv1.JSONSchemaProps{
			"stable": {Type: "string", Description: "Stable field.\n<gateway:experimental:description>\nOnly in experimental.\n</gateway:experimental:description>"},
			"exp":    {Type: "string", Description: "Experimental field.\n<gateway:experimental>"},
			"kind": {Type: "string", Description: "Kind.\n" +
				"<gateway:experimental:validation:Enum=A;B;C>\n" +
				"<gateway:standard:validation:XValidation:message=\"std\",rule=\"self != 'X'\">\n" +
				"<gateway:experimental:validation:Pattern=`^[A-C]$`>"},
			"addresses": {Type: "array", Description: "Addresses.\n<gateway:validateIPAddress>",
				Items: &apiextv1.JSONSchemaPropsOrArray{Schema: &apiextv1.JSONSchemaProps{Type: "object"}}},
		},
	}
}

func TestGatewayAPI(t *testing.T) {
	Convey("Standard", t, func() {
		s := schema()
		So(Standard().Modify("t", &s), ShouldBeNil)

		So(s.Properties, ShouldNotContainKey, "exp")
		So(s.Required, ShouldResemble, []string{"stable"})
		So(s.Description, ShouldEqual, "Type doc.")
		So(s.Properties["stable"].Description, ShouldEqual, "Stable field.")
		kind := s.Properties["kind"]
		So(kind.Enum, ShouldBeEmpty)
		So(kind.Pattern, ShouldBeEmpty)
		So(kind.XValidations, ShouldHaveLength, 1)
		So(kind.Description, ShouldEqual, "Kind.")
		So(s.Properties["addresses"].Items.Schema.OneOf, ShouldHaveLength, 2)
	})

	Convey("Experimental", t, func() {
		s := schema()
		So(Experimental().Modify("t", &s), ShouldBeNil)

		So(s.Properties, ShouldContainKey, "exp")
		So(s.Properties["stable"].Description, ShouldEqual, "Stable field.\n\nOnly in experimental.")
		kind := s.Properties["kind"]
		So(kind.Enum, ShouldHaveLength, 3)
		So(kind.Pattern, ShouldEqual, "^[A-C]$")
		So(kind.XValidations, ShouldBeEmpty)
	})

	Convey("broken tags are an error", t, func() {
		s := apiextv1.JSONSchemaProps{Description: "<gateway:standard:validation:Enum=bad-value>"}
		So(Standard().Modify("t", &s), ShouldNotBeNil)
	})
}
