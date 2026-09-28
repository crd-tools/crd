package crd

import (
	"io"
	"reflect"
	"strings"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"crd.tools/crd/registry"
)

// CRD собирает CustomResourceDefinition для Go-типа T
// Ошибки накапливаются: первая запоминается, дальнейшие вызовы ничего не делают,
// а Err, Build и Encode её возвращают
type CRD interface {
	// Group задаёт группу API
	Group(group string) CRD

	// Kind задаёт вид ресурса, ListKind строится как Kind + List
	Kind(kind string) CRD

	// Plural задаёт имя ресурса во множественном числе
	Plural(plural string) CRD

	// Singular задаёт имя в единственном числе, по умолчанию Kind в нижнем регистре
	Singular(singular string) CRD

	// ShortNames задаёт короткие имена для kubectl
	ShortNames(names ...string) CRD

	// Categories задаёт категории ресурса, например all для kubectl get all
	Categories(categories ...string) CRD

	// Scope задаёт область видимости, по умолчанию Namespaced
	Scope(scope apiextv1.ResourceScope) CRD

	// Version добавляет версию
	// Схема версии берётся из типа T или из VersionType, остальное задают опции
	Version(name string, served, storage bool, opts ...VersionOption) CRD

	// ConversionWebhook включает конверсию между версиями через вебхук
	// По умолчанию стратегия None: API-сервер меняет только apiVersion
	ConversionWebhook(cfg apiextv1.WebhookClientConfig, reviewVersions ...string) CRD

	// Annotation добавляет аннотацию CRD
	Annotation(key, value string) CRD

	// Label добавляет метку CRD
	Label(key, value string) CRD

	// Modifiers заменяет набор модификаторов схем этого определения
	// Модификаторы применяются при каждой сборке к схеме каждого типа ресурса и не влияют
	// ни на другие определения, ни на SchemaFor. Без аргументов набор очищается
	Modifiers(m ...Modifier) CRD

	// Err возвращает первую накопленную ошибку
	Err() error

	// GroupVersionKind возвращает GVK версии для регистрации типа в runtime.Scheme
	GroupVersionKind(version string) schema.GroupVersionKind

	// GroupVersionResource возвращает GVR версии для REST-клиентов и informer'ов
	GroupVersionResource(version string) schema.GroupVersionResource

	// Namespaced сообщает, живёт ли ресурс в namespace
	Namespaced() bool

	// HasStatus сообщает, включён ли у версии подресурс status,
	// то есть обновлять status нужно через UpdateStatus
	HasStatus(version string) bool

	// Build строит CustomResourceDefinition
	// Полная проверка результата кодом API-сервера — validation.Validate
	//
	// Рантайм и кеш, как SchemaFor, для типа каждой версии
	//
	//	InvalidArgument
	//	NotStruct
	//	коды построения схемы: FieldNotFound, RuleKindMismatch, UnsupportedType, ...
	Build() (*apiextv1.CustomResourceDefinition, error)

	// Encode строит CRD и пишет его в w в YAML
	//
	//	коды Build
	Encode(w io.Writer) error
}

// VersionOption настраивает версию CRD
type VersionOption func(v *version)

type version struct {
	name       string
	served     bool
	storage    bool
	typ        reflect.Type
	status     bool
	scale      *apiextv1.CustomResourceSubresourceScale
	columns    []apiextv1.CustomResourceColumnDefinition
	deprecated bool
	warning    *string
	err        error
}

// VersionType задаёт тип, схема которого используется в этой версии вместо T
func VersionType[T any]() VersionOption {
	return func(v *version) { v.typ = reflect.TypeFor[T]() }
}

// Status включает подресурс status: изменения status идут через /status,
// а изменения основного ресурса не трогают status
func Status() VersionOption {
	return func(v *version) { v.status = true }
}

// Scale включает подресурс scale для kubectl scale и HorizontalPodAutoscaler
// Пути задаются в виде JSON path, например .spec.replicas, labelSelectorPath может быть пустым
func Scale(specReplicasPath, statusReplicasPath, labelSelectorPath string) VersionOption {
	return func(v *version) {
		v.scale = &apiextv1.CustomResourceSubresourceScale{
			SpecReplicasPath:   specReplicasPath,
			StatusReplicasPath: statusReplicasPath,
		}
		if labelSelectorPath != "" {
			v.scale.LabelSelectorPath = &labelSelectorPath
		}
	}
}

// printerColumnTypes типы колонок, которые принимает API-сервер
var printerColumnTypes = map[string]bool{
	"integer": true, "number": true, "string": true, "boolean": true, "date": true,
}

// PrinterColumn добавляет колонку в вывод kubectl get
// typ — integer, number, string, boolean или date, jsonPath — например .spec.replicas
func PrinterColumn(name, typ, jsonPath string) VersionOption {
	return PrinterColumnDef(apiextv1.CustomResourceColumnDefinition{Name: name, Type: typ, JSONPath: jsonPath})
}

// PrinterColumnDef добавляет колонку со всеми настройками: описание, формат, приоритет
func PrinterColumnDef(def apiextv1.CustomResourceColumnDefinition) VersionOption {
	return func(v *version) {
		if !printerColumnTypes[def.Type] {
			v.fail(newError(InvalidArgument, "crd: version %s: column %s: unsupported type %q", v.name, def.Name, def.Type))
			return
		}
		v.columns = append(v.columns, def)
	}
}

// Deprecated помечает версию устаревшей, warning показывается клиентам, пустой — стандартный текст
func Deprecated(warning string) VersionOption {
	return func(v *version) {
		v.deprecated = true
		if warning != "" {
			v.warning = &warning
		}
	}
}

func (v *version) fail(err error) {
	if v.err == nil {
		v.err = err
	}
}

type resourceBuilder struct {
	typ         reflect.Type
	st          *state
	err         error
	group       string
	kind        string
	plural      string
	singular    string
	shortNames  []string
	categories  []string
	scope       apiextv1.ResourceScope
	versions    []*version
	conversion  *apiextv1.CustomResourceConversion
	annotations map[string]string
	labels      map[string]string
	modifiers   []Modifier
}

// New возвращает сборщик CRD для структуры T
// Схема T берётся так же, как в SchemaFor: из Describer, из снимков или выводится из Go-типа
//
//	NotStruct
func New[T any]() CRD {
	return newResource(std, reflect.TypeFor[T]())
}

func newResource(st *state, t reflect.Type) *resourceBuilder {
	r := &resourceBuilder{typ: t, st: st, scope: apiextv1.NamespaceScoped}
	if t.Kind() != reflect.Struct {
		r.err = newError(NotStruct, "crd.New: %s is not a struct", t)
	}
	return r
}

func (r *resourceBuilder) set(fn func()) CRD {
	if r.err == nil {
		fn()
	}
	return r
}

func (r *resourceBuilder) Group(v string) CRD    { return r.set(func() { r.group = v }) }
func (r *resourceBuilder) Kind(v string) CRD     { return r.set(func() { r.kind = v }) }
func (r *resourceBuilder) Plural(v string) CRD   { return r.set(func() { r.plural = v }) }
func (r *resourceBuilder) Singular(v string) CRD { return r.set(func() { r.singular = v }) }
func (r *resourceBuilder) ShortNames(v ...string) CRD {
	return r.set(func() { r.shortNames = append([]string(nil), v...) })
}
func (r *resourceBuilder) Categories(v ...string) CRD {
	return r.set(func() { r.categories = append([]string(nil), v...) })
}
func (r *resourceBuilder) Scope(v apiextv1.ResourceScope) CRD { return r.set(func() { r.scope = v }) }

func (r *resourceBuilder) Version(name string, served, storage bool, opts ...VersionOption) CRD {
	return r.set(func() {
		v := &version{name: name, served: served, storage: storage, typ: r.typ}
		for _, opt := range opts {
			opt(v)
		}
		if v.err == nil && v.typ.Kind() != reflect.Struct {
			v.err = newError(NotStruct, "crd: version %s: %s is not a struct", name, v.typ)
		}
		if v.err != nil {
			r.err = v.err
			return
		}
		r.versions = append(r.versions, v)
	})
}

func (r *resourceBuilder) ConversionWebhook(cfg apiextv1.WebhookClientConfig, reviewVersions ...string) CRD {
	return r.set(func() {
		if len(reviewVersions) == 0 {
			reviewVersions = []string{"v1"}
		}
		r.conversion = &apiextv1.CustomResourceConversion{
			Strategy: apiextv1.WebhookConverter,
			Webhook: &apiextv1.WebhookConversion{
				ClientConfig:             &cfg,
				ConversionReviewVersions: reviewVersions,
			},
		}
	})
}

func (r *resourceBuilder) Annotation(key, value string) CRD {
	return r.set(func() {
		if r.annotations == nil {
			r.annotations = map[string]string{}
		}
		r.annotations[key] = value
	})
}

func (r *resourceBuilder) Label(key, value string) CRD {
	return r.set(func() {
		if r.labels == nil {
			r.labels = map[string]string{}
		}
		r.labels[key] = value
	})
}

func (r *resourceBuilder) Modifiers(m ...Modifier) CRD {
	return r.set(func() {
		r.modifiers = nil
		for _, mod := range m {
			if mod == nil {
				r.err = newError(InvalidArgument, "crd: nil modifier")
				return
			}
			r.modifiers = append(r.modifiers, mod)
		}
	})
}

func (r *resourceBuilder) Err() error { return r.err }

func (r *resourceBuilder) GroupVersionKind(version string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: r.group, Version: version, Kind: r.kind}
}

func (r *resourceBuilder) GroupVersionResource(version string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: r.group, Version: version, Resource: r.plural}
}

func (r *resourceBuilder) Namespaced() bool {
	return r.scope != apiextv1.ClusterScoped
}

func (r *resourceBuilder) HasStatus(version string) bool {
	for _, v := range r.versions {
		if v.name == version {
			return v.status
		}
	}
	return false
}

func (r *resourceBuilder) Build() (*apiextv1.CustomResourceDefinition, error) {
	if r.err != nil {
		return nil, r.err
	}
	if err := r.check(); err != nil {
		return nil, err
	}

	singular := r.singular
	if singular == "" {
		singular = strings.ToLower(r.kind)
	}
	out := &apiextv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.plural + "." + r.group,
			Annotations: r.annotations,
			Labels:      r.labels,
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: r.group,
			Names: apiextv1.CustomResourceDefinitionNames{
				Kind:       r.kind,
				ListKind:   r.kind + "List",
				Plural:     r.plural,
				Singular:   singular,
				ShortNames: r.shortNames,
				Categories: r.categories,
			},
			Scope:      r.scope,
			Conversion: r.conversion,
		},
	}

	schemas := map[reflect.Type]apiextv1.JSONSchemaProps{}
	for _, v := range r.versions {
		root, ok := schemas[v.typ]
		if !ok {
			schema, err := r.st.resourceSchema(v.typ, modifyFunc(r.modifiers))
			if err != nil {
				return nil, wrap(err, "crd: version %s", v.name)
			}
			root = rootObject(*schema)
			schemas[v.typ] = root
		}
		s := *root.DeepCopy()
		ver := apiextv1.CustomResourceDefinitionVersion{
			Name:                     v.name,
			Served:                   v.served,
			Storage:                  v.storage,
			Deprecated:               v.deprecated,
			DeprecationWarning:       v.warning,
			Schema:                   &apiextv1.CustomResourceValidation{OpenAPIV3Schema: &s},
			AdditionalPrinterColumns: v.columns,
		}
		if v.status || v.scale != nil {
			ver.Subresources = &apiextv1.CustomResourceSubresources{Scale: v.scale}
			if v.status {
				ver.Subresources.Status = &apiextv1.CustomResourceSubresourceStatus{}
			}
		}
		out.Spec.Versions = append(out.Spec.Versions, ver)
	}
	return out, nil
}

// check проверяет обязательные настройки
func (r *resourceBuilder) check() error {
	switch {
	case r.group == "":
		return newError(InvalidArgument, "crd: empty group")
	case r.kind == "":
		return newError(InvalidArgument, "crd: empty kind")
	case r.plural == "":
		return newError(InvalidArgument, "crd: empty plural")
	case len(r.versions) == 0:
		return newError(InvalidArgument, "crd: no versions")
	}
	storage := 0
	names := map[string]bool{}
	for _, v := range r.versions {
		if names[v.name] {
			return newError(InvalidArgument, "crd: duplicate version %s", v.name)
		}
		names[v.name] = true
		if v.storage {
			storage++
		}
	}
	if storage != 1 {
		return newError(InvalidArgument, "crd: exactly one storage version required, got %d", storage)
	}
	return nil
}

func (r *resourceBuilder) Encode(w io.Writer) error {
	obj, err := r.Build()
	if err != nil {
		return err
	}
	data, err := Marshal(obj)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return newError(Internal, "crd: write: %s", err)
	}
	return nil
}

// Marshal кодирует CRD в YAML-документ, который начинается с ---
// Поле status не пишется: оно относится к состоянию ресурса в кластере, а не к манифесту
//
//	Internal
func Marshal(obj *apiextv1.CustomResourceDefinition) ([]byte, error) {
	if obj == nil {
		return nil, newError(InvalidArgument, "crd: marshal nil CRD")
	}
	data, err := yaml.Marshal(obj)
	if err != nil {
		return nil, newError(Internal, "crd: encode: %s", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, newError(Internal, "crd: encode: %s", err)
	}
	delete(m, "status")
	data, err = yaml.Marshal(m)
	if err != nil {
		return nil, newError(Internal, "crd: encode: %s", err)
	}
	return append([]byte("---\n"), data...), nil
}

// rootObject дополняет корень схемы полями, которые есть у любого ресурса
func rootObject(s apiextv1.JSONSchemaProps) apiextv1.JSONSchemaProps {
	s.Type = "object"
	if s.Properties == nil {
		s.Properties = map[string]apiextv1.JSONSchemaProps{}
	}
	for name, typ := range map[string]string{"apiVersion": "string", "kind": "string", "metadata": "object"} {
		if _, ok := s.Properties[name]; !ok {
			s.Properties[name] = apiextv1.JSONSchemaProps{Type: typ}
		}
	}
	return s
}

// resourceSchema возвращает схему типа ресурса с модификаторами определения
// Порядок как в Of: Describer, снимки, вывод из Go-типа
func (s *state) resourceSchema(t reflect.Type, modify registry.ModifyFunc) (*apiextv1.JSONSchemaProps, error) {
	key, ok := registry.KeyOf(t)
	if !ok {
		return nil, newError(InvalidArgument, "crd: %s is not a named type", t)
	}
	s.regMu.Lock()
	err := s.prepare(t)
	if err == nil {
		err = s.ensureAny(t)
	}
	s.regMu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.schemaWith(key, modify)
}
