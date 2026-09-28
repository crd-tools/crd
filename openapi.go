package crd

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	"crd.tools/crd/registry"
)

// OpenAPIVersion версия OpenAPI, которую строит OpenAPI
const OpenAPIVersion = "3.0.3"

// componentsPrefix начало ссылки на компонент
const componentsPrefix = "#/components/schemas/"

// Prefix это замена начала имени компонента
// Имя компонента строится из пути пакета и имени типа, / заменяется на точку:
// k8s.io/apimachinery/pkg/apis/meta/v1.Time → k8s.io.apimachinery.pkg.apis.meta.v1.Time.
// Замена From → To применяется к началу имени по границе точки, выбирается самая длинная
type Prefix struct {
	// From начало имени, которое заменяется
	From string `json:"from" yaml:"from"`

	// To на что заменяется, пусто — начало просто отрезается
	To string `json:"to" yaml:"to"`
}

// OpenAPIConfig это настройки документа OpenAPI
type OpenAPIConfig struct {
	// Title заголовок документа
	Title string

	// Version версия документа
	Version string

	// Prefixes замены начала имён компонентов
	Prefixes []Prefix
}

// OpenAPIDocument это документ OpenAPI v3 со схемами типов
type OpenAPIDocument struct {
	OpenAPI    string            `json:"openapi"`
	Info       OpenAPIInfo       `json:"info"`
	Paths      map[string]any    `json:"paths"`
	Components OpenAPIComponents `json:"components"`
}

// OpenAPIInfo это раздел info
type OpenAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// OpenAPIComponents это раздел components
type OpenAPIComponents struct {
	Schemas map[string]apiextv1.JSONSchemaProps `json:"schemas"`
}

// JSON кодирует документ в JSON
func (d *OpenAPIDocument) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, newError(Internal, "crd: encode OpenAPI: %s", err)
	}
	return data, nil
}

// YAML кодирует документ в YAML
func (d *OpenAPIDocument) YAML() ([]byte, error) {
	data, err := yaml.Marshal(d)
	if err != nil {
		return nil, newError(Internal, "crd: encode OpenAPI: %s", err)
	}
	return data, nil
}

// OpenAPI строит документ OpenAPI v3, где каждый именованный тип — отдельный компонент
//
// Поле с типом-компонентом становится ссылкой $ref, а если у поля есть свои свойства
// (описание, ограничения) — allOf со ссылкой и этими свойствами. Позиция, где правила
// родителя меняют вложенный тип, описывается на месте, чтобы общий компонент не нёс
// чужих правил. Схемы берутся по тем же приоритетам, что и в SchemaFor
//
// Рантайм и кеш, как SchemaFor, для типа каждой версии
//
//	InvalidArgument
//	коды построения схемы: FieldNotFound, RuleKindMismatch, UnsupportedType, ...
func OpenAPI(cfg OpenAPIConfig, defs ...CRD) (*OpenAPIDocument, error) {
	return std.openAPI(cfg, defs)
}

func (s *state) openAPI(cfg OpenAPIConfig, defs []CRD) (*OpenAPIDocument, error) {
	resources := make([]*resourceBuilder, 0, len(defs))
	for _, def := range defs {
		r, ok := def.(*resourceBuilder)
		if !ok || r == nil {
			return nil, newError(InvalidArgument, "crd.OpenAPI: definition %T is not made by crd.New", def)
		}
		if r.err != nil {
			return nil, r.err
		}
		resources = append(resources, r)
	}

	// регистрация типов с CRD и Rule и типов из очереди Of, как в SchemaFor
	for _, r := range resources {
		for _, t := range r.types() {
			if _, err := s.resourceSchema(t, nil); err != nil {
				return nil, err
			}
		}
	}
	reg, err := s.registry()
	if err != nil {
		return nil, err
	}

	s.regMu.Lock()
	defer s.regMu.Unlock()

	// имена общие для всех определений, компоненты строятся для каждого со своими модификаторами
	names := map[string]string{}
	keys := map[string]string{}
	merged := map[string]apiextv1.JSONSchemaProps{}
	owners := map[string]string{}
	for _, r := range resources {
		g := &openAPIGen{
			st:      s,
			reg:     reg,
			cfg:     cfg,
			modify:  modifyFunc(r.modifiers),
			names:   names,
			keys:    keys,
			types:   map[string]reflect.Type{},
			schemas: map[string]apiextv1.JSONSchemaProps{},
			busy:    map[string]bool{},
		}
		for _, t := range r.types() {
			name, err := g.typeComponent(t)
			if err != nil {
				return nil, err
			}
			g.schemas[name] = rootObject(g.schemas[name])
		}
		owner := r.kind + "." + r.group
		for name, schema := range g.schemas {
			if prev, ok := merged[name]; ok && !reflect.DeepEqual(prev, schema) {
				return nil, newError(InvalidArgument,
					"crd.OpenAPI: component %s differs between definitions %s and %s: use the same modifiers",
					name, owners[name], owner)
			}
			merged[name] = schema
			if _, ok := owners[name]; !ok {
				owners[name] = owner
			}
		}
	}

	return &OpenAPIDocument{
		OpenAPI:    OpenAPIVersion,
		Info:       OpenAPIInfo{Title: cfg.Title, Version: cfg.Version},
		Paths:      map[string]any{},
		Components: OpenAPIComponents{Schemas: merged},
	}, nil
}

// types возвращает типы ресурса: основной и типы версий, без повторов
func (r *resourceBuilder) types() []reflect.Type {
	seen := map[reflect.Type]bool{}
	var out []reflect.Type
	for _, t := range append([]reflect.Type{r.typ}, versionTypes(r.versions)...) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func versionTypes(versions []*version) []reflect.Type {
	out := make([]reflect.Type, 0, len(versions))
	for _, v := range versions {
		out = append(out, v.typ)
	}
	return out
}

// openAPIGen строит компоненты документа
type openAPIGen struct {
	st  *state
	reg *registry.Registry
	cfg OpenAPIConfig

	// modify модификаторы определения, для которого строятся компоненты
	modify registry.ModifyFunc

	// names ключ типа → имя компонента, keys имя компонента → ключ типа
	names map[string]string
	keys  map[string]string

	// types известные Go-типы по ключу, чтобы ссылку из снимка разбить так же, как тип из кода
	types map[string]reflect.Type

	schemas map[string]apiextv1.JSONSchemaProps

	// busy компоненты, которые строятся прямо сейчас, защита от циклов
	busy map[string]bool
}

// name возвращает имя компонента для ключа типа, проверяя совпадения
func (g *openAPIGen) name(key string) (string, error) {
	if n, ok := g.names[key]; ok {
		return n, nil
	}
	n := componentName(key, g.cfg.Prefixes)
	if other, ok := g.keys[n]; ok && other != key {
		return "", newError(InvalidArgument,
			"crd.OpenAPI: component name %s is the same for %s and %s, adjust the prefixes", n, other, key)
	}
	g.names[key] = n
	g.keys[n] = key
	return n, nil
}

// componentName строит имя компонента из ключа типа с заменой префиксов
func componentName(key string, prefixes []Prefix) string {
	name := strings.ReplaceAll(key, "/", ".")
	best := -1
	for i, p := range prefixes {
		if p.From == "" {
			continue
		}
		if name == p.From || strings.HasPrefix(name, p.From+".") {
			if best < 0 || len(p.From) > len(prefixes[best].From) {
				best = i
			}
		}
	}
	if best >= 0 {
		p := prefixes[best]
		rest := strings.TrimPrefix(name[len(p.From):], ".")
		switch {
		case p.To == "":
			name = rest
		case rest == "":
			name = p.To
		default:
			name = p.To + "." + rest
		}
	}
	return sanitizeName(name)
}

// sanitizeName оставляет в имени только символы, допустимые в имени компонента OpenAPI
func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// useTree сообщает, строить ли компонент типа по дереву билдера, а не брать схему из реестра
// Порядок как в SchemaFor: CRD и вывод из Go-типа дают дерево, Rule, RegisterSchema и снимок — нет
func (g *openAPIGen) useTree(t reflect.Type, key string) bool {
	switch {
	case g.st.fromTree[key]:
		return true
	case g.reg.HasRuntime(key):
		return false
	case reflect.PointerTo(t).Implements(describerType):
		return true
	case g.reg.Has(key):
		return false
	}
	return t.Kind() == reflect.Struct
}

// typeComponent строит компонент для Go-типа и возвращает его имя
func (g *openAPIGen) typeComponent(t reflect.Type) (string, error) {
	t = deref(t)
	key, ok := registry.KeyOf(t)
	if !ok {
		return "", newError(InvalidArgument, "crd.OpenAPI: %s is not a named type", t)
	}
	g.types[key] = t
	if !g.useTree(t, key) {
		return g.keyComponent(key)
	}
	name, err := g.name(key)
	if err != nil {
		return "", err
	}
	if _, done := g.schemas[name]; done || g.busy[key] {
		return name, nil
	}
	g.busy[key] = true
	defer delete(g.busy, key)

	root, err := newBuilder(g.st).tree(reflect.New(t))
	if err != nil {
		return "", wrap(err, "crd.OpenAPI")
	}
	schema, err := g.renderNode(root, true)
	if err != nil {
		return "", err
	}
	if g.modify != nil {
		if err := g.modify(key, &schema); err != nil {
			return "", newError(ModifierFailed, "crd.OpenAPI: %s: %s", key, err)
		}
	}
	g.schemas[name] = schema
	return name, nil
}

// keyComponent строит компонент по ключу типа
// Если Go-тип известен и строится деревом, компонент строится по дереву, иначе — по схеме из реестра
func (g *openAPIGen) keyComponent(key string) (string, error) {
	if t, ok := g.types[key]; ok && g.useTree(t, key) {
		return g.typeComponent(t)
	}
	name, err := g.name(key)
	if err != nil {
		return "", err
	}
	if _, done := g.schemas[name]; done || g.busy[key] {
		return name, nil
	}
	g.busy[key] = true
	defer delete(g.busy, key)

	raw, err := g.reg.Modified(key, g.modify)
	if err != nil {
		return "", wrap(err, "crd.OpenAPI")
	}
	schema, err := g.convertRefs(*raw)
	if err != nil {
		return "", err
	}
	g.schemas[name] = schema
	return name, nil
}

// convertRefs переписывает ключи реестра в $ref на компоненты, соседние свойства — в allOf
func (g *openAPIGen) convertRefs(s apiextv1.JSONSchemaProps) (apiextv1.JSONSchemaProps, error) {
	var walkErr error
	registry.Walk(&s, func(n *apiextv1.JSONSchemaProps) bool {
		if walkErr != nil {
			return false
		}
		if n.Ref == nil || *n.Ref == "" || strings.HasPrefix(*n.Ref, componentsPrefix) {
			return true
		}
		name, err := g.keyComponent(*n.Ref)
		if err != nil {
			walkErr = err
			return false
		}
		siblings := *n.DeepCopy()
		siblings.Ref = nil
		*n = refWith(name, siblings)
		return false
	})
	return s, walkErr
}

// refWith возвращает ссылку на компонент, а при наличии своих свойств — allOf со ссылкой
// В OpenAPI 3.0 свойства рядом с $ref игнорируются, поэтому они выносятся рядом с allOf
func refWith(name string, siblings apiextv1.JSONSchemaProps) apiextv1.JSONSchemaProps {
	ref := componentsPrefix + name
	if reflect.ValueOf(siblings).IsZero() {
		return apiextv1.JSONSchemaProps{Ref: &ref}
	}
	siblings.AllOf = append([]apiextv1.JSONSchemaProps{{Ref: &ref}}, siblings.AllOf...)
	return siblings
}

// renderNode собирает схему узла, вынося именованные типы в компоненты
func (g *openAPIGen) renderNode(n *node, root bool) (apiextv1.JSONSchemaProps, error) {
	s := *n.schema.DeepCopy()
	switch n.kind {
	case kindRef:
		name, err := g.refComponent(n)
		if err != nil {
			return s, err
		}
		return refWith(name, s), nil
	case kindObject:
		if !root && named(n.goType) {
			return g.objectPosition(n)
		}
		if len(n.fields) > 0 {
			s.Properties = make(map[string]apiextv1.JSONSchemaProps, len(n.fields))
			for fieldName, f := range n.fields {
				child, err := g.renderNode(f, false)
				if err != nil {
					return s, err
				}
				s.Properties[fieldName] = child
			}
		}
		var required []string
		for fieldName, ok := range n.required {
			if ok {
				required = append(required, fieldName)
			}
		}
		sort.Strings(required)
		s.Required = required
		for _, in := range n.inlines {
			g.types[in.key] = in.goType
			name, err := g.keyComponent(in.key)
			if err != nil {
				return s, err
			}
			s.AllOf = append(s.AllOf, refWith(name, apiextv1.JSONSchemaProps{}))
		}
	case kindArray:
		items, err := g.renderNode(n.items, false)
		if err != nil {
			return s, err
		}
		s.Items = &apiextv1.JSONSchemaPropsOrArray{Schema: &items}
	case kindMap:
		values, err := g.renderNode(n.values, false)
		if err != nil {
			return s, err
		}
		s.AdditionalProperties = &apiextv1.JSONSchemaPropsOrBool{Allows: true, Schema: &values}
	case kindFixed:
		// готовые правила могут ссылаться на типы через Of
		return g.convertRefs(s)
	}
	return s, nil
}

// refComponent возвращает компонент для узла-ссылки
func (g *openAPIGen) refComponent(n *node) (string, error) {
	if n.goType != nil {
		if key, ok := registry.KeyOf(n.goType); ok && key == n.ref {
			g.types[key] = n.goType
		}
	}
	return g.keyComponent(n.ref)
}

// objectPosition описывает вложенную структуру в позиции поля
// Если правила родителя не меняют её поля, это ссылка на общий компонент, иначе — описание на месте
func (g *openAPIGen) objectPosition(n *node) (apiextv1.JSONSchemaProps, error) {
	name, err := g.typeComponent(n.goType)
	if err != nil {
		return apiextv1.JSONSchemaProps{}, err
	}
	here, err := g.renderNode(n, true)
	if err != nil {
		return here, err
	}
	common := g.schemas[name]
	if !sameStructure(here, common) {
		return here, nil
	}
	return refWith(name, overlay(here, common)), nil
}

// sameStructure сообщает, что у схем одинаковое содержимое: поля, обязательность, элементы
func sameStructure(a, b apiextv1.JSONSchemaProps) bool {
	return reflect.DeepEqual(a.Properties, b.Properties) &&
		reflect.DeepEqual(a.Required, b.Required) &&
		reflect.DeepEqual(a.AdditionalProperties, b.AdditionalProperties) &&
		reflect.DeepEqual(a.Items, b.Items)
}

// overlay возвращает свойства позиции, которых нет в общем компоненте
func overlay(here, common apiextv1.JSONSchemaProps) apiextv1.JSONSchemaProps {
	out := here
	hv := reflect.ValueOf(here)
	cv := reflect.ValueOf(common)
	ov := reflect.ValueOf(&out).Elem()
	for i := 0; i < hv.NumField(); i++ {
		if reflect.DeepEqual(hv.Field(i).Interface(), cv.Field(i).Interface()) {
			ov.Field(i).Set(reflect.Zero(hv.Field(i).Type()))
		}
	}
	return out
}

// named сообщает, что тип можно вынести в компонент
func named(t reflect.Type) bool {
	return t != nil && t.Name() != "" && t.PkgPath() != ""
}
