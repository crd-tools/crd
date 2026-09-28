package registry

import (
	"fmt"
	"reflect"
	"sort"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// overlay накладывает свойства поля на схему типа, на который поле ссылается
// Документация берётся из поля, остальное сливается как в allOf
func overlay(target apiextv1.JSONSchemaProps, field apiextv1.JSONSchemaProps) (apiextv1.JSONSchemaProps, error) {
	field.Ref = nil
	desc, title, docs, example := field.Description, field.Title, field.ExternalDocs, field.Example
	field.Description, field.Title, field.ExternalDocs, field.Example = "", "", nil, nil

	if err := mergeInto(&target, field); err != nil {
		return target, err
	}
	if desc != "" {
		target.Description = desc
	}
	if title != "" {
		target.Title = title
	}
	if docs != nil {
		target.ExternalDocs = docs
	}
	if example != nil {
		target.Example = example
	}
	return target, nil
}

// flattenAllOf вливает allOf во владельца на всех уровнях схемы
func flattenAllOf(s *apiextv1.JSONSchemaProps) error {
	var err error
	Walk(s, func(node *apiextv1.JSONSchemaProps) bool {
		if err != nil {
			return false
		}
		if len(node.AllOf) == 0 {
			return true
		}
		items := node.AllOf
		node.AllOf = nil
		for _, item := range items {
			if err = mergeInto(node, item); err != nil {
				return false
			}
			// объект без своего описания берёт описание встроенного типа
			if node.Description == "" {
				node.Description = item.Description
			}
		}
		return true
	})
	return err
}

// mergeInto сливает src в dst, повторяя семантику сплющивания controller-tools
// Конфликтующие ограничения поднимаются в allOf, чтобы применились оба
func mergeInto(dst *apiextv1.JSONSchemaProps, src apiextv1.JSONSchemaProps) error {
	for _, item := range src.AllOf {
		if err := mergeInto(dst, item); err != nil {
			return err
		}
	}

	dstVal := reflect.ValueOf(dst).Elem()
	srcVal := reflect.ValueOf(src)
	typ := dstVal.Type()

	var dstRest, srcRest apiextv1.JSONSchemaProps
	dstRestVal := reflect.ValueOf(&dstRest).Elem()
	srcRestVal := reflect.ValueOf(&srcRest).Elem()
	hoisted := false

	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		switch name {
		case "AllOf", "Title", "Description", "Example", "ExternalDocs":
			continue
		case "Enum":
			if len(src.Enum) > 0 && len(dst.Enum) == 0 {
				dst.Enum = append([]apiextv1.JSON(nil), src.Enum...)
			}
			continue
		}

		sf, df := srcVal.Field(i), dstVal.Field(i)
		if sf.IsZero() {
			continue
		}
		if df.IsZero() {
			df.Set(sf)
			continue
		}
		if reflect.DeepEqual(sf.Interface(), df.Interface()) {
			continue
		}

		switch name {
		case "Properties":
			for k, v := range src.Properties {
				cur, ok := dst.Properties[k]
				if !ok {
					dst.Properties[k] = v
					continue
				}
				if err := mergeInto(&cur, v); err != nil {
					return err
				}
				dst.Properties[k] = cur
			}
		case "Required":
			dst.Required = append(dst.Required, src.Required...)
		case "AdditionalProperties":
			if src.AdditionalProperties.Schema == nil {
				continue
			}
			if dst.AdditionalProperties.Schema == nil {
				dst.AdditionalProperties.Schema = &apiextv1.JSONSchemaProps{}
			}
			if err := mergeInto(dst.AdditionalProperties.Schema, *src.AdditionalProperties.Schema); err != nil {
				return err
			}
		case "XPreserveUnknownFields", "XMapType":
			df.Set(sf)
		case "XValidations":
			dst.XValidations = append(append(apiextv1.ValidationRules(nil), src.XValidations...), dst.XValidations...)
		case "Type":
			return fmt.Errorf("%w: types %s and %s", ErrConflict, dst.Type, src.Type)
		default:
			srcRestVal.Field(i).Set(sf)
			dstRestVal.Field(i).Set(df)
			df.Set(reflect.Zero(df.Type()))
			hoisted = true
		}
	}

	if hoisted {
		dst.AllOf = append(dst.AllOf, dstRest, srcRest)
	}
	dst.Required = uniqSorted(dst.Required)
	return nil
}

func uniqSorted(items []string) []string {
	if len(items) == 0 {
		return items
	}
	set := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := set[item]; ok {
			continue
		}
		set[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
