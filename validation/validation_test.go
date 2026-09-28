package validation_test

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"crd.tools/crd"
	"crd.tools/crd/validation"
)

type appSpec struct {
	Replicas int32             `json:"replicas"`
	Selector string            `json:"selector,omitempty"`
	Template map[string]string `json:"template,omitempty"`
	Pod      podTemplate       `json:"pod"`
	Extra    any               `json:"extra,omitempty"`
}

type podTemplate struct {
	Image string `json:"image"`
}

func (s *appSpec) CRD(b crd.Builder) error {
	b.Integer(&s.Replicas).Minimum(0)
	b.Object(&s.Pod).PreserveUnknownFields()
	b.Raw(&s.Extra).EmbeddedResource()
	return nil
}

type appStatus struct {
	Replicas int32  `json:"replicas,omitempty"`
	Selector string `json:"selector,omitempty"`
}

type app struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   appSpec   `json:"spec"`
	Status appStatus `json:"status,omitempty"`
}

// appV1beta1 это старая версия со своей схемой
type appV1beta1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Size int32 `json:"size"`
}

func build(c crd.CRD) *apiextv1.CustomResourceDefinition {
	obj, err := c.Build()
	So(err, ShouldBeNil)
	return obj
}

func TestValidate(t *testing.T) {
	ctx := context.Background()

	Convey("a full CRD passes the API server validation", t, func() {
		obj := build(crd.New[app]().
			Group("example.com").Kind("App").Plural("apps").Categories("all").
			Version("v1", true, true,
				crd.Status(),
				crd.Scale(".spec.replicas", ".status.replicas", ".status.selector"),
				crd.PrinterColumn("Replicas", "integer", ".spec.replicas"),
				crd.PrinterColumnDef(apiextv1.CustomResourceColumnDefinition{
					Name: "Age", Type: "date", JSONPath: ".metadata.creationTimestamp", Priority: 1,
				}),
			).
			Version("v1beta1", true, false, crd.VersionType[appV1beta1](), crd.Deprecated("use v1")).
			ConversionWebhook(apiextv1.WebhookClientConfig{
				Service: &apiextv1.ServiceReference{Namespace: "system", Name: "webhook", Path: ptr("/convert")},
			}))
		So(validation.Validate(ctx, obj), ShouldBeNil)

		v1, v1beta1 := obj.Spec.Versions[0], obj.Spec.Versions[1]
		So(v1.Subresources.Status, ShouldNotBeNil)
		So(v1.Subresources.Scale.SpecReplicasPath, ShouldEqual, ".spec.replicas")
		So(v1.AdditionalPrinterColumns, ShouldHaveLength, 2)
		So(v1.Schema.OpenAPIV3Schema.Properties, ShouldContainKey, "spec")
		spec := v1.Schema.OpenAPIV3Schema.Properties["spec"]
		So(*spec.Properties["pod"].XPreserveUnknownFields, ShouldBeTrue)
		So(spec.Properties["extra"].XEmbeddedResource, ShouldBeTrue)
		So(spec.Properties["extra"].Type, ShouldEqual, "object")

		So(v1beta1.Deprecated, ShouldBeTrue)
		So(v1beta1.Schema.OpenAPIV3Schema.Properties, ShouldContainKey, "size")
		So(v1beta1.Schema.OpenAPIV3Schema.Properties, ShouldNotContainKey, "spec")
		So(obj.Spec.Conversion.Strategy, ShouldEqual, apiextv1.WebhookConverter)
		So(obj.Spec.Conversion.Webhook.ConversionReviewVersions, ShouldResemble, []string{"v1"})
	})

	Convey("the API server rules catch mistakes the builder cannot", t, func() {
		obj := build(crd.New[app]().
			Group("example.com").Kind("App").Plural("apps").
			Version("v1", true, true, crd.Scale(".spec.replicas", ".spec.replicas", "")))
		err := validation.Validate(ctx, obj)
		So(crd.Code(err), ShouldEqual, crd.InvalidCRD)
		So(err.Error(), ShouldContainSubstring, "statusReplicasPath")

		So(crd.Code(validation.Validate(ctx, nil)), ShouldEqual, crd.InvalidArgument)
	})
}

func ptr[T any](v T) *T { return &v }
