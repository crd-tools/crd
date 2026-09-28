// Package gatewayapi содержит модификаторы схем для Gateway API (sigs.k8s.io/gateway-api)
//
// Gateway API кроме маркеров kubebuilder использует свои теги в комментариях вида
// <gateway:experimental:validation:...>. controller-tools их не понимает и оставляет в
// описаниях, а генератор Gateway API применяет их постобработкой и получает CRD двух каналов.
// Модификаторы повторяют эту постобработку (tools/generator в репозитории Gateway API):
//   - Standard — удаляет поля с <gateway:experimental>, применяет проверки канала standard;
//   - Experimental — оставляет все поля, применяет проверки канала experimental.
//
// В обоих каналах теги вычищаются из описаний.
//
//	var HTTPRoute = crd.New[HTTPRouteWrapper]().
//		Group("gateway.networking.k8s.io").Kind("HTTPRoute").Plural("httproutes").
//		Version("v1", true, true, crd.Status()).
//		Modifiers(gatewayapi.Standard())
package gatewayapi

import (
	"fmt"
	"regexp"
	"strings"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"crd.tools/crd"
)

// Standard возвращает модификатор стандартного канала Gateway API
func Standard() crd.Modifier {
	return gatewayAPI{channel: "standard"}
}

// Experimental возвращает модификатор экспериментального канала Gateway API
func Experimental() crd.Modifier {
	return gatewayAPI{channel: "experimental"}
}

var (
	gwExperimentalDescRe = regexp.MustCompile(`\n*` + regexp.QuoteMeta("<gateway:experimental:description>") +
		`(?s:(.*?))` + regexp.QuoteMeta("</gateway:experimental:description>") + `\n*`)
	gwExcludeRe = regexp.MustCompile(`\n*` + regexp.QuoteMeta("<gateway:util:excludeFromCRD>") +
		`(?s:(.*?))` + regexp.QuoteMeta("</gateway:util:excludeFromCRD>") + `\n*`)
	gwTagRe      = regexp.MustCompile(`<gateway:.*>`)
	gwNewlinesRe = regexp.MustCompile(`\n\n\n+`)
)

// gatewayAPI это постобработка схем Gateway API для одного канала
type gatewayAPI struct {
	channel string
}

// Modify обрабатывает схему типа: сам узел, его свойства и элементы
func (g gatewayAPI) Modify(key string, s *apiextv1.JSONSchemaProps) error {
	return g.node(key, s)
}

// node обрабатывает узел схемы и его вложенные узлы
func (g gatewayAPI) node(name string, s *apiextv1.JSONSchemaProps) error {
	if strings.Contains(s.Description, "<gateway:validateIPAddress>") && s.Items != nil && s.Items.Schema != nil {
		s.Items.Schema.OneOf = ipAddressOneOf()
	}
	// так же поступает генератор Gateway API: controller-tools ставит для date-time тип object
	if s.Format == "date-time" {
		s.Type = "string"
	}
	if err := g.validations(name, s); err != nil {
		return err
	}
	desc, err := g.description(name, s.Description)
	if err != nil {
		return err
	}
	s.Description = desc

	if len(s.Properties) > 0 {
		for prop := range s.Properties {
			p := s.Properties[prop]
			if g.channel == "standard" && strings.Contains(p.Description, "<gateway:experimental>") {
				delete(s.Properties, prop)
				s.Required = removeString(s.Required, prop)
				continue
			}
			if err := g.node(prop, &p); err != nil {
				return err
			}
			s.Properties[prop] = p
		}
	} else if s.Items != nil && s.Items.Schema != nil {
		return g.node(name, s.Items.Schema)
	}
	return nil
}

// validations применяет проверки своего канала из тегов в описании
func (g gatewayAPI) validations(name string, s *apiextv1.JSONSchemaProps) error {
	prefix := fmt.Sprintf("<gateway:%s:validation:", g.channel)
	expected := strings.Count(s.Description, prefix)
	if expected == 0 {
		return nil
	}
	valid := 0

	enumRe := regexp.MustCompile(regexp.QuoteMeta(prefix) + `Enum=([A-Za-z;]*)>`)
	for _, m := range enumRe.FindAllStringSubmatch(s.Description, 64) {
		valid++
		s.Enum = nil
		for _, v := range strings.Split(m[1], ";") {
			s.Enum = append(s.Enum, apiextv1.JSON{Raw: []byte(`"` + v + `"`)})
		}
	}

	celRe := regexp.MustCompile(regexp.QuoteMeta(prefix) + `XValidation:message="([^"]*)",rule="([^"]*)">`)
	for _, m := range celRe.FindAllStringSubmatch(s.Description, 64) {
		valid++
		s.XValidations = append(s.XValidations, apiextv1.ValidationRule{Message: m[1], Rule: m[2]})
	}

	patternRe := regexp.MustCompile(regexp.QuoteMeta(prefix) + "Pattern=`([^`]*)`")
	if m := patternRe.FindAllStringSubmatch(s.Description, 64); len(m) == 1 && s.Pattern == "" {
		valid++
		s.Pattern = m[0][1]
	}

	if valid < expected {
		return fmt.Errorf("%s: found %d Gateway API validation tags, only %d are valid", name, expected, valid)
	}
	return nil
}

// description вычищает теги Gateway API из описания
func (g gatewayAPI) description(name, desc string) (string, error) {
	if g.channel == "standard" && strings.Contains(desc, "<gateway:experimental:description>") {
		if len(gwExperimentalDescRe.FindStringSubmatch(desc)) != 2 {
			return "", fmt.Errorf("%s: invalid <gateway:experimental:description> tag", name)
		}
		desc = gwExperimentalDescRe.ReplaceAllString(desc, "\n\n")
	} else {
		desc = strings.ReplaceAll(desc, "<gateway:experimental:description>", "")
		desc = strings.ReplaceAll(desc, "</gateway:experimental:description>", "")
	}
	if strings.Contains(desc, "<gateway:util:excludeFromCRD>") {
		if len(gwExcludeRe.FindStringSubmatch(desc)) != 2 {
			return "", fmt.Errorf("%s: invalid <gateway:util:excludeFromCRD> tag", name)
		}
		desc = gwExcludeRe.ReplaceAllString(desc, "\n\n\n")
	}
	desc = gwTagRe.ReplaceAllLiteralString(desc, "")
	desc = gwNewlinesRe.ReplaceAllString(desc, "\n\n\n")
	return strings.Trim(desc, "\n"), nil
}

// ipAddressOneOf это проверка адреса для полей с <gateway:validateIPAddress>
func ipAddressOneOf() []apiextv1.JSONSchemaProps {
	ipType := apiextv1.JSON{Raw: []byte(`"IPAddress"`)}
	return []apiextv1.JSONSchemaProps{
		{Properties: map[string]apiextv1.JSONSchemaProps{
			"type":  {Enum: []apiextv1.JSON{ipType}},
			"value": {AnyOf: []apiextv1.JSONSchemaProps{{Format: "ipv4"}, {Format: "ipv6"}}},
		}},
		{Properties: map[string]apiextv1.JSONSchemaProps{
			"type": {Not: &apiextv1.JSONSchemaProps{Enum: []apiextv1.JSON{ipType}}},
		}},
	}
}

func removeString(items []string, v string) []string {
	out := items[:0]
	for _, item := range items {
		if item != v {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
