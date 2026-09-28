package crd

import (
	"encoding/json"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Ветки комбинаторов несут только ограничения значения: у них нет типа и вложенных
// комбинаторов, как того требует структурная схема CRD

// StringBranch это ветка комбинатора для строки
type StringBranch interface {
	MinLength(n int64) StringBranch
	MaxLength(n int64) StringBranch
	Pattern(p string) StringBranch
	Format(f string) StringBranch
	Enum(values ...string) StringBranch
}

// IntegerBranch это ветка комбинатора для целого числа
type IntegerBranch interface {
	Minimum(n int64) IntegerBranch
	Maximum(n int64) IntegerBranch
	MultipleOf(n int64) IntegerBranch
	Enum(values ...int64) IntegerBranch
}

// NumberBranch это ветка комбинатора для числа с плавающей точкой
type NumberBranch interface {
	Minimum(n float64) NumberBranch
	Maximum(n float64) NumberBranch
	MultipleOf(n float64) NumberBranch
	Enum(values ...float64) NumberBranch
}

// ObjectBranch это ветка комбинатора для объекта
// Набор обязательных полей задаёт взаимоисключающие или альтернативные варианты
type ObjectBranch interface {
	RequiredFields(names ...string) ObjectBranch
}

// branchRef возвращает схему ветки по месту, а не по указателю
// Срез веток может перевыделиться при добавлении следующей ветки
type branchRef func() *apiextv1.JSONSchemaProps

func branch(list *[]apiextv1.JSONSchemaProps) branchRef {
	*list = append(*list, apiextv1.JSONSchemaProps{})
	i := len(*list) - 1
	return func() *apiextv1.JSONSchemaProps { return &(*list)[i] }
}

func not(s *apiextv1.JSONSchemaProps) branchRef {
	if s.Not == nil {
		s.Not = &apiextv1.JSONSchemaProps{}
	}
	return func() *apiextv1.JSONSchemaProps { return s.Not }
}

type stringBranch struct{ s branchRef }

func (b stringBranch) MinLength(n int64) StringBranch { b.s().MinLength = &n; return b }
func (b stringBranch) MaxLength(n int64) StringBranch { b.s().MaxLength = &n; return b }
func (b stringBranch) Pattern(p string) StringBranch  { b.s().Pattern = p; return b }
func (b stringBranch) Format(f string) StringBranch   { b.s().Format = f; return b }
func (b stringBranch) Enum(values ...string) StringBranch {
	for _, v := range values {
		b.s().Enum = append(b.s().Enum, enumValue(v))
	}
	return b
}

type integerBranch struct{ s branchRef }

func (b integerBranch) Minimum(n int64) IntegerBranch {
	f := float64(n)
	b.s().Minimum = &f
	return b
}
func (b integerBranch) Maximum(n int64) IntegerBranch {
	f := float64(n)
	b.s().Maximum = &f
	return b
}
func (b integerBranch) MultipleOf(n int64) IntegerBranch {
	f := float64(n)
	b.s().MultipleOf = &f
	return b
}
func (b integerBranch) Enum(values ...int64) IntegerBranch {
	for _, v := range values {
		b.s().Enum = append(b.s().Enum, enumValue(v))
	}
	return b
}

type numberBranch struct{ s branchRef }

func (b numberBranch) Minimum(n float64) NumberBranch    { b.s().Minimum = &n; return b }
func (b numberBranch) Maximum(n float64) NumberBranch    { b.s().Maximum = &n; return b }
func (b numberBranch) MultipleOf(n float64) NumberBranch { b.s().MultipleOf = &n; return b }
func (b numberBranch) Enum(values ...float64) NumberBranch {
	for _, v := range values {
		b.s().Enum = append(b.s().Enum, enumValue(v))
	}
	return b
}

type objectBranch struct{ s branchRef }

func (b objectBranch) RequiredFields(names ...string) ObjectBranch {
	b.s().Required = append(b.s().Required, names...)
	return b
}

// enumValue кодирует значение ветки, для строк и чисел кодирование не даёт ошибок
func enumValue(v any) apiextv1.JSON {
	data, _ := json.Marshal(v)
	return apiextv1.JSON{Raw: data}
}
