package crd

import (
	"encoding/json"
	"reflect"
	"time"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// quantityPattern шаблон resource.Quantity, как в controller-tools
const quantityPattern = `^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`

// knownTypes это схемы типов, у которых своё JSON-представление
// Структура таких типов не совпадает с тем, как они выглядят в JSON
var knownTypes = map[reflect.Type]func() apiextv1.JSONSchemaProps{
	reflect.TypeFor[time.Time](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{Type: "string", Format: "date-time"}
	},
	reflect.TypeFor[json.RawMessage](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{XPreserveUnknownFields: ptr(true)}
	},
	reflect.TypeFor[metav1.Time](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{Type: "string", Format: "date-time"}
	},
	reflect.TypeFor[metav1.MicroTime](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{Type: "string", Format: "date-time"}
	},
	reflect.TypeFor[metav1.Duration](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{Type: "string"}
	},
	reflect.TypeFor[resource.Quantity](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{
			XIntOrString: true,
			AnyOf: []apiextv1.JSONSchemaProps{
				{Type: "integer"},
				{Type: "string"},
			},
			Pattern: quantityPattern,
		}
	},
	reflect.TypeFor[intstr.IntOrString](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{XIntOrString: true}
	},
	reflect.TypeFor[runtime.RawExtension](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{Type: "object", XPreserveUnknownFields: ptr(true)}
	},
	reflect.TypeFor[apiextv1.JSON](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{XPreserveUnknownFields: ptr(true)}
	},
	reflect.TypeFor[metav1.ObjectMeta](): func() apiextv1.JSONSchemaProps {
		return apiextv1.JSONSchemaProps{Type: "object"}
	},
}

func ptr[T any](v T) *T {
	return &v
}

// Методы, которыми типы описывают свою OpenAPI-схему по соглашению kube-openapi
// Их реализуют metav1.Time, metav1.Duration, intstr.IntOrString, resource.Quantity и сторонние типы
type (
	openAPISchemaTyper interface {
		OpenAPISchemaType() []string
	}
	openAPISchemaFormatter interface {
		OpenAPISchemaFormat() string
	}
	openAPIOneOfTyper interface {
		OpenAPIV3OneOfTypes() []string
	}
)

// builtinSchema возвращает схему типа, которую не нужно выводить из его полей:
// сначала из таблицы известных типов, затем из методов kube-openapi
func builtinSchema(t reflect.Type) (apiextv1.JSONSchemaProps, bool, error) {
	if fn, ok := knownTypes[t]; ok {
		return fn(), true, nil
	}
	return openAPISchema(t)
}

// openAPISchema строит схему по методам kube-openapi
// Значение, которое может быть и числом, и строкой, получает x-kubernetes-int-or-string
func openAPISchema(t reflect.Type) (apiextv1.JSONSchemaProps, bool, error) {
	v := reflect.New(t).Interface()
	typer, ok := v.(openAPISchemaTyper)
	if !ok {
		return apiextv1.JSONSchemaProps{}, false, nil
	}
	var format string
	if f, ok := v.(openAPISchemaFormatter); ok {
		format = f.OpenAPISchemaFormat()
	}
	types := typer.OpenAPISchemaType()
	if o, ok := v.(openAPIOneOfTyper); ok && len(o.OpenAPIV3OneOfTypes()) > 0 {
		types = o.OpenAPIV3OneOfTypes()
	}
	if format == "int-or-string" || intOrString(types) {
		return apiextv1.JSONSchemaProps{XIntOrString: true}, true, nil
	}
	if len(types) != 1 {
		return apiextv1.JSONSchemaProps{}, true, newError(UnsupportedType,
			"%s: OpenAPI types %v cannot be expressed in a CRD schema", t, types)
	}
	return apiextv1.JSONSchemaProps{Type: types[0], Format: format}, true, nil
}

// intOrString сообщает, что среди типов есть и число, и строка
func intOrString(types []string) bool {
	var num, str bool
	for _, t := range types {
		switch t {
		case "integer", "number":
			num = true
		case "string":
			str = true
		}
	}
	return num && str
}
