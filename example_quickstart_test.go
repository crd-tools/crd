package crd_test

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"crd.tools/crd"
)

// Двойник быстрого старта из README

type AppSpec struct {
	Replicas int32  `json:"replicas"`
	Image    string `json:"image"`
}

func (s *AppSpec) CRD(b crd.Builder) error {
	b.Integer(&s.Replicas).Minimum(0).Maximum(10)
	b.NonEmptyString(&s.Image).Description("образ контейнера")
	return nil
}

type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AppSpec `json:"spec"`
}

var AppResource = crd.New[App]().
	Group("example.com").Kind("App").Plural("apps").
	Version("v1", true, true)

// Example_quickStart это быстрый старт из README: тип, определение CRD и схема spec
func Example_quickStart() {
	obj, err := AppResource.Build()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	spec := obj.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
	out, _ := yaml.Marshal(spec)
	fmt.Print(string(out))
	fmt.Println(AppResource.GroupVersionResource("v1"))
	// Output:
	// properties:
	//   image:
	//     description: образ контейнера
	//     minLength: 1
	//     type: string
	//   replicas:
	//     format: int32
	//     maximum: 10
	//     minimum: 0
	//     type: integer
	// required:
	// - image
	// - replicas
	// type: object
	// example.com/v1, Resource=apps
}
