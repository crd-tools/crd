package crd

import (
	"reflect"
	"sort"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// kind это вид узла схемы
type kind int

const (
	// kindScalar строка, число или булево значение
	kindScalar kind = iota

	// kindObject структура с полями
	kindObject

	// kindArray срез или массив
	kindArray

	// kindMap мапа со строковыми ключами
	kindMap

	// kindRef ссылка на схему другого типа в реестре
	kindRef

	// kindFixed готовая схема: известный тип, Raw, IntOrString или правило из Use
	kindFixed
)

// node это узел схемы, который строится сверху вниз и настраивается билдером
type node struct {
	kind kind

	// goType тип значения без указателей, по нему проверяется вид правила
	goType reflect.Type

	// schema собственные свойства узла, для ссылки это свойства рядом с $ref
	schema apiextv1.JSONSchemaProps

	// ref ключ типа для kindRef
	ref string

	// fields поля объекта по json-имени
	fields map[string]*node

	// required обязательность полей объекта по json-имени
	required map[string]bool

	// items схема элемента массива
	items *node

	// values схема значения мапы
	values *node

	// inlines встроенные поля с меткой снимка: их схема подключается через allOf
	inlines []inlineRef
}

// inlineRef это встроенный тип, схема которого берётся по ключу реестра
type inlineRef struct {
	key    string
	goType reflect.Type
}

func newObjectNode(t reflect.Type) *node {
	return &node{
		kind:     kindObject,
		goType:   t,
		schema:   apiextv1.JSONSchemaProps{Type: "object"},
		fields:   map[string]*node{},
		required: map[string]bool{},
	}
}

// render собирает схему узла и всех вложенных узлов
func (n *node) render() apiextv1.JSONSchemaProps {
	s := *n.schema.DeepCopy()
	switch n.kind {
	case kindRef:
		ref := n.ref
		s.Ref = &ref
	case kindObject:
		if len(n.fields) > 0 {
			s.Properties = make(map[string]apiextv1.JSONSchemaProps, len(n.fields))
			for name, f := range n.fields {
				s.Properties[name] = f.render()
			}
		}
		var required []string
		for name, ok := range n.required {
			if ok {
				required = append(required, name)
			}
		}
		sort.Strings(required)
		s.Required = required
		for _, in := range n.inlines {
			ref := in.key
			s.AllOf = append(s.AllOf, apiextv1.JSONSchemaProps{Ref: &ref})
		}
	case kindArray:
		items := n.items.render()
		s.Items = &apiextv1.JSONSchemaPropsOrArray{Schema: &items}
	case kindMap:
		values := n.values.render()
		s.AdditionalProperties = &apiextv1.JSONSchemaPropsOrBool{Allows: true, Schema: &values}
	}
	return s
}

// replace превращает узел в готовую схему, сохраняя документацию узла
func (n *node) replace(s apiextv1.JSONSchemaProps) {
	if s.Description == "" {
		s.Description = n.schema.Description
	}
	n.kind = kindFixed
	n.schema = s
	n.ref = ""
	n.fields, n.required, n.items, n.values = nil, nil, nil, nil
}
