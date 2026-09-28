package crd

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"crd.tools/crd/registry"
)

// Modifier правит схему одного типа при сборке CRD или OpenAPI
//
// Модификаторы задаются на определении через CRD.Modifiers и применяются к схеме каждого
// типа, входящего в ресурс: к копии схемы со ссылками $ref, до их раскрытия. Результат не
// попадает в общий кеш, поэтому разные наборы модификаторов дают независимые результаты,
// а SchemaFor и другие определения их не видят.
//
// Типичное применение — теги проекта в описаниях, которые не понимает controller-tools,
// например каналы Gateway API: пакет crd.tools/crd/modifiers/gatewayapi
type Modifier interface {
	Modify(key string, s *apiextv1.JSONSchemaProps) error
}

// ModifierFunc позволяет использовать функцию как Modifier
type ModifierFunc func(key string, s *apiextv1.JSONSchemaProps) error

// Modify вызывает функцию
func (f ModifierFunc) Modify(key string, s *apiextv1.JSONSchemaProps) error {
	return f(key, s)
}

// modifyFunc собирает модификаторы в одну функцию для реестра, nil — без модификаций
func modifyFunc(modifiers []Modifier) registry.ModifyFunc {
	if len(modifiers) == 0 {
		return nil
	}
	return func(key string, s *apiextv1.JSONSchemaProps) error {
		for _, m := range modifiers {
			if err := m.Modify(key, s); err != nil {
				return err
			}
		}
		return nil
	}
}
