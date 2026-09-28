// Package crd даёт доступ к OpenAPI-схемам Go-типов в рантайме
//
// Схемы приходят из двух источников:
//   - снимки из kubebuilder-маркеров, подключаются через SetDERSource;
//   - правила из кода: типы с Describer или Ruler и готовые схемы через RegisterSchema.
//
// Схема из кода перекрывает снимок того же типа, в том числе там, где на тип ссылаются через $ref
package crd

import (
	"io/fs"
	"reflect"
	"strings"
	"sync"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"crd.tools/crd/registry"
)

// Rule это правило, описывающее схему значения
type Rule interface {
	// Schema возвращает схему значения
	Schema() (apiextv1.JSONSchemaProps, error)
}

// Ruler это тип, который сам описывает свою схему, например type ClientID string
type Ruler interface {
	// Rule возвращает правило для значений типа
	Rule() (Rule, error)
}

var std = newState()

// SetDERSource подключает каталоги снимков из сгенерированных пакетов
// Снимки читаются лениво, при первом запросе схемы, поэтому порядок init не важен
func SetDERSource(sources ...fs.FS) {
	std.setDERSource(sources...)
}

// Register регистрирует схему типа значения v, который реализует Describer или Ruler
// Типы полей с Describer или Ruler регистрируются автоматически
//
// Рантайм: всегда вызывает CRD или Rule заново и перезаписывает схему
func Register(v any) error {
	return std.register(v)
}

// SetKubebuildTag задаёт метку полей, схема которых берётся из снимков
// По умолчанию crd:"kubebuild"
func SetKubebuildTag(key, value string) {
	std.setKubebuildTag(key, value)
}

// RegisterSchema регистрирует готовую схему для типа значения v
//
// Схема из RegisterSchema, зарегистрированная раньше первого обращения к типу,
// заменяет его CRD или Rule: они для этого типа вызываться не будут
func RegisterSchema(v any, schema apiextv1.JSONSchemaProps) error {
	return std.registerSchema(v, schema)
}

// Schema возвращает схему с раскрытыми ссылками по ключу вида путь/пакета.Тип
//
// Только кеш: ищет среди уже зарегистрированных схем и снимков, CRD и Rule не вызывает.
// По строке нельзя найти Go-тип, поэтому тип с CRD или Rule, который ещё не встречался,
// придёт из снимка или не найдётся. Для таких типов используйте SchemaFor или
// регистрируйте их в init своего пакета
//
//	NotFound
func Schema(key string) (*apiextv1.JSONSchemaProps, error) {
	return std.schema(key)
}

// SchemaFor возвращает схему с раскрытыми ссылками для типа T
//
// Рантайм и кеш: при первом обращении обходит граф Go-типов T, включая поля типов из снимков,
// и вызывает CRD или Rule для всех найденных типов, которые их реализуют. Дальше берёт из кеша
//
//	NotFound
func SchemaFor[T any]() (*apiextv1.JSONSchemaProps, error) {
	return std.schemaFor(reflect.TypeFor[T]())
}

// Of возвращает правило-значение, которое ссылается на схему типа T
// Нужно там, где правило требуется значением: Items, Values, NewArray, NewMap
// Тип регистрируется при разрешении схемы: из Describer или Ruler, из снимков,
// а обычная структура описывается выводом из Go-типа
//
// Рантайм и кеш: CRD или Rule типа вызываются один раз, при первом разрешении схемы
func Of[T any]() Rule {
	return typeRule{st: std, t: reflect.TypeFor[T]()}
}

type typeRule struct {
	st *state
	t  reflect.Type
}

// Schema возвращает ссылку на тип и ставит тип в очередь регистрации
//
//	InvalidArgument
func (r typeRule) Schema() (apiextv1.JSONSchemaProps, error) {
	t := deref(r.t)
	key, ok := registry.KeyOf(t)
	if !ok {
		return apiextv1.JSONSchemaProps{}, newError(InvalidArgument, "crd.Of: %s is not a named type", t)
	}
	r.st.queueType(t)
	return apiextv1.JSONSchemaProps{Ref: &key}, nil
}

// state это реестр, ещё не прочитанные источники снимков и очередь типов на регистрацию
type state struct {
	// mu защищает pending, queue и метку
	mu      sync.Mutex
	reg     *registry.Registry
	pending []fs.FS
	queue   []reflect.Type

	// regMu сериализует построение схем из кода
	regMu sync.Mutex

	// building типы, схема которых строится прямо сейчас, защищено regMu
	building map[reflect.Type]bool

	// prepared типы, граф которых уже обойдён, защищено regMu
	prepared map[reflect.Type]bool

	// fromTree ключи, схема которых в реестре построена деревом билдера,
	// а не взята из Rule или RegisterSchema, защищено regMu
	fromTree map[string]bool

	tagKey   string
	tagValue string
}

func newState() *state {
	return &state{
		reg:      registry.New(),
		building: map[reflect.Type]bool{},
		prepared: map[reflect.Type]bool{},
		fromTree: map[string]bool{},
		tagKey:   "crd",
		tagValue: "kubebuild",
	}
}

func (s *state) setKubebuildTag(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tagKey, s.tagValue = key, value
}

// kubebuildTagged сообщает, помечено ли поле меткой снимков
func (s *state) kubebuildTagged(f reflect.StructField) bool {
	s.mu.Lock()
	key, want := s.tagKey, s.tagValue
	s.mu.Unlock()
	value, ok := f.Tag.Lookup(key)
	if !ok {
		return false
	}
	for _, item := range strings.Split(value, ",") {
		if strings.TrimSpace(item) == want {
			return true
		}
	}
	return false
}

func (s *state) setDERSource(sources ...fs.FS) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, src := range sources {
		if src != nil {
			s.pending = append(s.pending, src)
		}
	}
}

func (s *state) queueType(t reflect.Type) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = append(s.queue, t)
}

// registry подключает ожидающие источники и возвращает реестр
func (s *state) registry() (*registry.Registry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.pending) > 0 {
		if err := s.reg.AddFS(s.pending[0]); err != nil {
			return nil, newError(SourceLoad, "crd: load DER source: %s", err)
		}
		s.pending = s.pending[1:]
	}
	return s.reg, nil
}

// describedByCode сообщает, описывает ли тип свою схему сам
func describedByCode(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	return pt.Implements(describerType) || pt.Implements(rulerType)
}

func (s *state) register(v any) error {
	if v == nil {
		return newError(InvalidArgument, "crd: register nil")
	}
	t := deref(reflect.TypeOf(v))
	s.regMu.Lock()
	defer s.regMu.Unlock()
	return s.build(t, false)
}

// ensure регистрирует тип с Describer или Ruler, если он ещё не зарегистрирован
// Вызывается под regMu
func (s *state) ensure(t reflect.Type) error {
	key, ok := registry.KeyOf(t)
	if !ok {
		return newError(InvalidArgument, "crd: %s is not a named type", t)
	}
	if s.reg.HasRuntime(key) {
		return nil
	}
	return s.build(t, false)
}

// ensureAny делает так, чтобы у типа была схема в реестре
// Порядок: Describer или Ruler, затем снимки, затем вывод из Go-типа для структуры
// Вызывается под regMu
func (s *state) ensureAny(t reflect.Type) error {
	if describedByCode(t) {
		return s.ensure(t)
	}
	key, ok := registry.KeyOf(t)
	if !ok {
		return newError(InvalidArgument, "crd: %s is not a named type", t)
	}
	reg, err := s.registry()
	if err != nil {
		return err
	}
	if reg.Has(key) {
		return nil
	}
	if schema, ok, err := builtinSchema(t); ok || err != nil {
		if err != nil {
			return err
		}
		return wrap(s.reg.Set(key, schema), "crd")
	}
	if t.Kind() != reflect.Struct {
		return newError(NotDescribed, "crd: %s has no schema: implement Describer or Ruler or register a schema", t)
	}
	return s.build(t, true)
}

// build строит схему типа и регистрирует её
// plain разрешает описать структуру без Describer выводом из Go-типа:
// Register требует Describer или Ruler, а Of и сборка CRD допускают обычные структуры
// Вызывается под regMu
func (s *state) build(t reflect.Type, plain bool) error {
	key, ok := registry.KeyOf(t)
	if !ok {
		return newError(InvalidArgument, "crd: %s is not a named type", t)
	}
	if s.building[t] {
		return newError(RecursiveType, "crd: recursive schema reference to %s", t)
	}
	s.building[t] = true
	defer delete(s.building, t)

	pv := reflect.New(t)
	var schema apiextv1.JSONSchemaProps
	fromTree := false
	switch v := pv.Interface().(type) {
	case Ruler:
		rule, err := v.Rule()
		if err != nil {
			return wrap(err, "crd: %s.Rule", t)
		}
		if rule == nil {
			return newError(InvalidRule, "crd: %s.Rule: nil rule", t)
		}
		schema, err = rule.Schema()
		if err != nil {
			return wrap(err, "crd: %s.Rule", t)
		}
	default:
		if t.Kind() != reflect.Struct {
			if _, ok := v.(Describer); ok {
				return newError(NotStruct, "crd: %s implements Describer but is not a struct", t)
			}
			return newError(NotDescribed, "crd: %s implements neither Describer nor Ruler", t)
		}
		if _, ok := v.(Describer); !ok && !plain {
			return newError(NotDescribed, "crd: %s implements neither Describer nor Ruler", t)
		}
		var err error
		schema, err = newBuilder(s).build(pv)
		if err != nil {
			return wrap(err, "crd")
		}
		fromTree = true
	}
	if err := s.reg.Set(key, schema); err != nil {
		return wrap(err, "crd")
	}
	s.fromTree[key] = fromTree
	return nil
}

func (s *state) registerSchema(v any, schema apiextv1.JSONSchemaProps) error {
	if v == nil {
		return newError(InvalidArgument, "crd: register nil")
	}
	key, ok := registry.KeyOf(reflect.TypeOf(v))
	if !ok {
		return newError(InvalidArgument, "crd: %T is not a named type", v)
	}
	if err := s.reg.Set(key, schema); err != nil {
		return wrap(err, "crd")
	}
	s.regMu.Lock()
	s.fromTree[key] = false
	s.regMu.Unlock()
	return nil
}

// flush регистрирует типы из очереди Of
func (s *state) flush() error {
	s.regMu.Lock()
	defer s.regMu.Unlock()
	for {
		s.mu.Lock()
		queue := s.queue
		s.queue = nil
		s.mu.Unlock()
		if len(queue) == 0 {
			return nil
		}
		for _, t := range queue {
			if err := s.ensureAny(t); err != nil {
				return err
			}
		}
	}
}

func (s *state) schema(key string) (*apiextv1.JSONSchemaProps, error) {
	return s.schemaWith(key, nil)
}

// schemaWith раскрывает схему, применяя modify к схеме каждого типа, nil — общий кеш
func (s *state) schemaWith(key string, modify registry.ModifyFunc) (*apiextv1.JSONSchemaProps, error) {
	reg, err := s.registry()
	if err != nil {
		return nil, err
	}
	if err := s.flush(); err != nil {
		return nil, err
	}
	schema, err := reg.ResolveWith(key, modify)
	if err != nil {
		return nil, wrap(err, "crd")
	}
	return schema, nil
}

// schemaFor возвращает схему типа, предварительно регистрируя все типы с Describer
// или Ruler в его графе. Тип без схемы описывается так же, как в New и Of
func (s *state) schemaFor(t reflect.Type) (*apiextv1.JSONSchemaProps, error) {
	t = deref(t)
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
	return s.schema(key)
}

// prepare обходит граф Go-типов и регистрирует типы с Describer или Ruler
// Нужен, чтобы правило из кода перекрывало снимок и внутри $ref, даже если тип попал
// в граф через тип из снимков, для которого CRD не вызывается
// Вызывается под regMu
func (s *state) prepare(t reflect.Type) error {
	t = deref(t)
	if s.prepared[t] {
		return nil
	}
	s.prepared[t] = true
	if describedByCode(t) {
		if err := s.ensure(t); err != nil {
			delete(s.prepared, t)
			return err
		}
	}
	if _, ok, _ := builtinSchema(t); ok {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if err := s.prepare(t.Field(i).Type); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		return s.prepare(t.Elem())
	}
	return nil
}
