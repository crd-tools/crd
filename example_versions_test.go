package crd_test

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"crd.tools/crd"
)

// versionsSpec это спецификация текущей версии
type versionsSpec struct {
	Replicas int32 `json:"replicas"`
}

func (s *versionsSpec) CRD(b crd.Builder) error {
	b.Integer(&s.Replicas).Minimum(0)
	return nil
}

// versionsStatus это состояние ресурса
type versionsStatus struct {
	Replicas int32 `json:"replicas,omitempty"`
}

// versionsApp это текущая версия ресурса
type versionsApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   versionsSpec   `json:"spec"`
	Status versionsStatus `json:"status,omitempty"`
}

// versionsAppV1beta1 это старая версия со своей схемой
type versionsAppV1beta1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Size int32 `json:"size"`
}

// Example_versions показывает версии с подресурсами, колонками kubectl и своей схемой
// v1 берёт схему из типа New, v1beta1 — из VersionType и помечена устаревшей
func Example_versions() {
	obj, err := crd.New[versionsApp]().
		Group("example.com").Kind("App").Plural("apps").
		Version("v1", true, true,
			crd.Status(),
			crd.Scale(".spec.replicas", ".status.replicas", ""),
			crd.PrinterColumn("Replicas", "integer", ".spec.replicas"),
		).
		Version("v1beta1", true, false,
			crd.VersionType[versionsAppV1beta1](),
			crd.Deprecated("используйте v1"),
		).
		Build()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, v := range obj.Spec.Versions {
		v.Schema = nil // схемы показаны в других примерах
		out, _ := yaml.Marshal(v)
		fmt.Print(string(out))
		fmt.Println("---")
	}
	// Output:
	// additionalPrinterColumns:
	// - jsonPath: .spec.replicas
	//   name: Replicas
	//   type: integer
	// name: v1
	// served: true
	// storage: true
	// subresources:
	//   scale:
	//     specReplicasPath: .spec.replicas
	//     statusReplicasPath: .status.replicas
	//   status: {}
	// ---
	// deprecated: true
	// deprecationWarning: используйте v1
	// name: v1beta1
	// served: true
	// storage: false
	// ---
}
