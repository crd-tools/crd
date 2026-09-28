package crd_test

import (
	"fmt"

	"crd.tools/crd"
)

// errorsSpec содержит ошибки описания
type errorsSpec struct {
	N int    `json:"n"`
	S string `json:"s"`
}

func (s *errorsSpec) CRD(b crd.Builder) error {
	var local string
	b.String(&s.N)
	b.String(&local)
	return nil
}

// Example_errors показывает ошибки описания: они копятся, указывают на строку и несут код
// Код составной ошибки — код первой из них
func Example_errors() {
	_, err := crd.SchemaFor[errorsSpec]()
	fmt.Println(crd.Code(err) == crd.RuleKindMismatch)
	fmt.Println(err)
	// Output:
	// true
	// crd: errorsSpec.CRD:
	//   example_errors_test.go:17: errorsSpec.N: string rule is not applicable to int
	//   example_errors_test.go:18: *string is not a pointer to a field of errorsSpec
}
