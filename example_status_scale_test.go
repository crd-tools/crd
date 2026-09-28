package crd_test

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"crd.tools/crd"
)

// Двойник примера из docs/status-and-scale.md

// WorkerSpec это желаемое состояние
type WorkerSpec struct {
	Replicas int32                `json:"replicas"`
	Selector metav1.LabelSelector `json:"selector"`
	Image    string               `json:"image"`
}

func (s *WorkerSpec) CRD(b crd.Builder) error {
	b.Integer(&s.Replicas).Minimum(0).Maximum(100).Description("желаемое число реплик")
	b.String(&s.Image).MinLength(1)
	return nil
}

// WorkerStatus это фактическое состояние, его пишет контроллер
type WorkerStatus struct {
	Replicas int32  `json:"replicas,omitempty"`
	Selector string `json:"selector,omitempty"`
}

func (s *WorkerStatus) CRD(b crd.Builder) error {
	b.Integer(&s.Replicas).Minimum(0).Description("фактическое число реплик")
	b.String(&s.Selector).Description("селектор подов строкой, для HPA")
	return nil
}

// Worker это ресурс
type Worker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerSpec   `json:"spec"`
	Status WorkerStatus `json:"status,omitempty"`
}

// WorkerResource это определение CRD
var WorkerResource = crd.New[Worker]().
	Group("example.com").Kind("Worker").Plural("workers").
	Version("v1", true, true,
		crd.Status(),
		crd.Scale(".spec.replicas", ".status.replicas", ".status.selector"),
		crd.PrinterColumn("Desired", "integer", ".spec.replicas"),
		crd.PrinterColumn("Ready", "integer", ".status.replicas"),
	)

// Example_statusAndScale это ресурс с подресурсами status и scale из docs/status-and-scale.md
func Example_statusAndScale() {
	obj, err := WorkerResource.Build()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	v := obj.Spec.Versions[0]
	out, _ := yaml.Marshal(map[string]any{
		"subresources":             v.Subresources,
		"additionalPrinterColumns": v.AdditionalPrinterColumns,
	})
	fmt.Print(string(out))
	// Output:
	// additionalPrinterColumns:
	// - jsonPath: .spec.replicas
	//   name: Desired
	//   type: integer
	// - jsonPath: .status.replicas
	//   name: Ready
	//   type: integer
	// subresources:
	//   scale:
	//     labelSelectorPath: .status.selector
	//     specReplicasPath: .spec.replicas
	//     statusReplicasPath: .status.replicas
	//   status: {}
}
