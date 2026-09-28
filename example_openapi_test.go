package crd_test

import (
	"fmt"
	"sort"

	"sigs.k8s.io/yaml"

	"crd.tools/crd"
)

// Example_openAPI строит документ OpenAPI по определению из docs/status-and-scale.md
// Каждый именованный тип — отдельный компонент, имена сокращены заменами префиксов
func Example_openAPI() {
	doc, err := crd.OpenAPI(crd.OpenAPIConfig{
		Title:   "workers",
		Version: "v1",
		Prefixes: []crd.Prefix{
			{From: "crd.tools.crd_test", To: ""},
			{From: "k8s.io.apimachinery.pkg", To: "apimachinery"},
		},
	}, WorkerResource)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	var names []string
	for name := range doc.Components.Schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
	fmt.Println("---")
	out, _ := yaml.Marshal(doc.Components.Schemas["WorkerSpec"])
	fmt.Print(string(out))
	// Output:
	// Worker
	// WorkerSpec
	// WorkerStatus
	// apimachinery.apis.meta.v1.LabelSelector
	// apimachinery.apis.meta.v1.LabelSelectorRequirement
	// ---
	// properties:
	//   image:
	//     minLength: 1
	//     type: string
	//   replicas:
	//     description: желаемое число реплик
	//     format: int32
	//     maximum: 100
	//     minimum: 0
	//     type: integer
	//   selector:
	//     $ref: '#/components/schemas/apimachinery.apis.meta.v1.LabelSelector'
	// required:
	// - image
	// - replicas
	// - selector
	// type: object
}
