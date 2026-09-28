package crd

import (
	"encoding"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"crd.tools/crd/registry"
)

// Describer это тип с полями, который описывает свою схему
// CRD вызывается на экземпляре, который создаёт билдер: в нём можно брать адреса полей,
// но не читать их значения
type Describer interface {
	CRD(b Builder) error
}

// Builder это описание полей структуры внутри Describer.CRD
// Поле передаётся указателем на него: &s.Field, &s.Nested.Field, &s.Ptr.Field
// Вид правила проверяется по Go-типу поля в момент вызова
type Builder interface {
	// Self настраивает сам объект, для которого вызван CRD
	Self() Object

	// String настраивает строковое поле
	String(field any) StringField

	// Integer настраивает целочисленное поле
	Integer(field any) IntegerField

	// Number настраивает поле с плавающей точкой
	Number(field any) NumberField

	// Bool настраивает булево поле
	Bool(field any) BoolField

	// Array настраивает поле-срез или массив
	Array(field any) ArrayField

	// Map настраивает поле-мапу
	Map(field any) MapField

	// Object настраивает поле-структуру
	Object(field any) ObjectField

	// Raw делает поле произвольным JSON
	Raw(field any) RawField

	// IntOrString делает поле целым числом или строкой
	IntOrString(field any) IntOrStringField

	// Use заменяет схему поля готовым правилом
	Use(field any, r Rule) Field

	// K8sName настраивает поле как имя в стиле DNS
	K8sName(field any) StringField

	// HTTPSURL настраивает поле как строку, начинающуюся с https://
	HTTPSURL(field any) StringField

	// NonEmptyString настраивает поле как непустую строку
	NonEmptyString(field any) StringField

	// Port настраивает поле как TCP-порт 1..65535
	Port(field any) IntegerField

	// Percent настраивает поле как процент 0..100
	Percent(field any) IntegerField

	// DurationSeconds настраивает поле как длительность в секундах в границах [lo, hi]
	DurationSeconds(field any, lo, hi int64) IntegerField
}

var (
	describerType = reflect.TypeFor[Describer]()
	rulerType     = reflect.TypeFor[Ruler]()
	marshalerType = reflect.TypeFor[json.Marshaler]()
	textType      = reflect.TypeFor[encoding.TextMarshaler]()
)

// fieldKey это адрес поля вместе с его типом
// Тип нужен, потому что первое поле структуры имеет тот же адрес, что и сама структура
type fieldKey struct {
	addr uintptr
	typ  reflect.Type
}

// fieldRef это поле в дереве узлов
type fieldRef struct {
	parent *node
	name   string
	node   *node
	goName string
	inline bool
}

// builder строит схему одного типа
type builder struct {
	st       *state
	root     *node
	rootName string
	fields   map[fieldKey]*fieldRef
	errs     []error
	stack    map[reflect.Type]bool
}

func newBuilder(st *state) *builder {
	return &builder{
		st:     st,
		fields: map[fieldKey]*fieldRef{},
		stack:  map[reflect.Type]bool{},
	}
}

// build строит схему структуры по дереву узлов
func (b *builder) build(pv reflect.Value) (apiextv1.JSONSchemaProps, error) {
	root, err := b.tree(pv)
	if err != nil {
		return apiextv1.JSONSchemaProps{}, err
	}
	return root.render(), nil
}

// tree строит дерево узлов по экземпляру структуры и вызывает CRD
func (b *builder) tree(pv reflect.Value) (*node, error) {
	t := pv.Type().Elem()
	b.rootName = t.Name()
	b.root = newObjectNode(t)
	b.stack[t] = true
	if err := b.deriveStruct(pv.Elem(), b.root, true, t.Name()); err != nil {
		return nil, err
	}
	if d, ok := pv.Interface().(Describer); ok {
		if err := d.CRD(b); err != nil {
			return nil, wrap(err, "%s.CRD", t.Name())
		}
	}
	if len(b.errs) > 0 {
		return nil, joinErrors(t.Name()+".CRD", b.errs)
	}
	return b.root, nil
}

// deriveStruct строит поля объекта
// register включает запоминание адресов полей, только для экземпляра, переданного в CRD
func (b *builder) deriveStruct(v reflect.Value, obj *node, register bool, owner string) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, opts := parseJSON(f)
		if name == "-" && !opts.dash {
			continue
		}
		base := deref(f.Type)
		inline := opts.inline || (f.Anonymous && name == "" && base.Kind() == reflect.Struct)
		if !f.IsExported() && !(inline && f.Anonymous) {
			continue
		}
		fv := v.Field(i)
		goName := owner + "." + f.Name

		if inline && b.st.kubebuildTagged(f) {
			// встроенный тип из снимка: его поля вливаются в объект через allOf при раскрытии
			key, ok := registry.KeyOf(base)
			if !ok {
				return newError(UnsupportedType, "%s: kubebuild tag requires a named type, got %s", goName, base)
			}
			obj.inlines = append(obj.inlines, inlineRef{key: key, goType: base})
			if register && fv.CanAddr() {
				b.fields[fieldKey{fv.Addr().Pointer(), f.Type}] = &fieldRef{parent: obj, goName: goName, inline: true}
			}
			continue
		}
		if inline {
			sv, reg := b.structValue(fv, base, register)
			if err := b.deriveStruct(sv, obj, reg, owner); err != nil {
				return err
			}
			if register && fv.CanAddr() {
				b.fields[fieldKey{fv.Addr().Pointer(), f.Type}] = &fieldRef{parent: obj, goName: goName, inline: true}
			}
			continue
		}
		if name == "" {
			name = f.Name
		}
		n, err := b.deriveField(fv, f, register, goName)
		if err != nil {
			return wrap(err, "%s", goName)
		}
		obj.fields[name] = n
		obj.required[name] = !opts.omitempty && !opts.omitzero
		if register && fv.CanAddr() {
			b.fields[fieldKey{fv.Addr().Pointer(), f.Type}] = &fieldRef{parent: obj, name: name, node: n, goName: goName}
		}
	}
	return nil
}

func (b *builder) deriveField(fv reflect.Value, f reflect.StructField, register bool, goName string) (*node, error) {
	if b.st.kubebuildTagged(f) {
		return refChain(f.Type)
	}
	return b.deriveValue(fv, f.Type, register, goName)
}

// structValue возвращает значение структуры, выделяя память под нулевой указатель
// Адреса полей запоминаются, только если структура лежит в экземпляре, переданном в CRD
func (b *builder) structValue(fv reflect.Value, base reflect.Type, register bool) (reflect.Value, bool) {
	if fv.Kind() != reflect.Pointer {
		return fv, register && fv.CanAddr()
	}
	if fv.IsNil() {
		if !register || !fv.CanSet() {
			return reflect.New(base).Elem(), false
		}
		fv.Set(reflect.New(base))
	}
	return fv.Elem(), register
}

// deriveValue строит узел по Go-типу, v может быть недействительным для элементов контейнеров
func (b *builder) deriveValue(v reflect.Value, t reflect.Type, register bool, owner string) (*node, error) {
	base := deref(t)
	// схема из кода важнее встроенного знания о типе
	if key, ok := b.selfDescribed(base); ok {
		if err := b.st.ensure(base); err != nil {
			return nil, err
		}
		return &node{kind: kindRef, goType: base, ref: key}, nil
	}
	if schema, ok, err := builtinSchema(base); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return &node{kind: kindFixed, goType: base, schema: schema}, nil
	}
	if base.Implements(marshalerType) || reflect.PointerTo(base).Implements(marshalerType) ||
		base.Implements(textType) || reflect.PointerTo(base).Implements(textType) {
		return nil, newError(UnsupportedType, "%s has its own JSON encoding: implement Ruler, register a schema or mark the field with the kubebuild tag", base)
	}

	switch base.Kind() {
	case reflect.String:
		return scalar(base, "string", ""), nil
	case reflect.Bool:
		return scalar(base, "boolean", ""), nil
	case reflect.Int, reflect.Uint:
		return scalar(base, "integer", ""), nil
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint8, reflect.Uint16:
		return scalar(base, "integer", "int32"), nil
	case reflect.Int64, reflect.Uint32, reflect.Uint64:
		return scalar(base, "integer", "int64"), nil
	case reflect.Float32:
		return scalar(base, "number", "float"), nil
	case reflect.Float64:
		return scalar(base, "number", "double"), nil
	case reflect.Interface:
		return &node{kind: kindFixed, goType: base, schema: apiextv1.JSONSchemaProps{XPreserveUnknownFields: ptr(true)}}, nil
	case reflect.Slice:
		if base.Elem().Kind() == reflect.Uint8 {
			return scalar(base, "string", "byte"), nil
		}
		fallthrough
	case reflect.Array:
		items, err := b.deriveValue(reflect.Value{}, base.Elem(), false, owner+"[]")
		if err != nil {
			return nil, err
		}
		return &node{kind: kindArray, goType: base, schema: apiextv1.JSONSchemaProps{Type: "array"}, items: items}, nil
	case reflect.Map:
		switch base.Key().Kind() {
		case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		default:
			return nil, newError(UnsupportedType, "map key %s is not supported", base.Key())
		}
		values, err := b.deriveValue(reflect.Value{}, base.Elem(), false, owner+"{}")
		if err != nil {
			return nil, err
		}
		return &node{kind: kindMap, goType: base, schema: apiextv1.JSONSchemaProps{Type: "object"}, values: values}, nil
	case reflect.Struct:
		if b.stack[base] {
			return nil, newError(RecursiveType, "recursive type %s is not supported", base)
		}
		b.stack[base] = true
		defer delete(b.stack, base)
		var sv reflect.Value
		if v.IsValid() {
			sv, register = b.structValue(v, base, register)
		} else {
			sv, register = reflect.New(base).Elem(), false
		}
		obj := newObjectNode(base)
		if err := b.deriveStruct(sv, obj, register, owner); err != nil {
			return nil, err
		}
		return obj, nil
	}
	return nil, newError(UnsupportedType, "type %s cannot be mapped to a schema", base)
}

// selfDescribed сообщает, описывает ли тип свою схему сам или через RegisterSchema
func (b *builder) selfDescribed(t reflect.Type) (string, bool) {
	key, ok := registry.KeyOf(t)
	if !ok {
		return "", false
	}
	pt := reflect.PointerTo(t)
	if t.Implements(describerType) || pt.Implements(describerType) ||
		t.Implements(rulerType) || pt.Implements(rulerType) {
		return key, true
	}
	return key, b.st.reg.HasRuntime(key)
}

// refChain строит ссылку на снимок с учётом срезов и мап вокруг типа
func refChain(t reflect.Type) (*node, error) {
	base := deref(t)
	switch base.Kind() {
	case reflect.Slice, reflect.Array:
		items, err := refChain(base.Elem())
		if err != nil {
			return nil, err
		}
		return &node{kind: kindArray, goType: base, schema: apiextv1.JSONSchemaProps{Type: "array"}, items: items}, nil
	case reflect.Map:
		values, err := refChain(base.Elem())
		if err != nil {
			return nil, err
		}
		return &node{kind: kindMap, goType: base, schema: apiextv1.JSONSchemaProps{Type: "object"}, values: values}, nil
	}
	key, ok := registry.KeyOf(base)
	if !ok {
		return nil, newError(UnsupportedType, "kubebuild tag requires a named type, got %s", base)
	}
	return &node{kind: kindRef, goType: base, ref: key}, nil
}

func scalar(t reflect.Type, typ, format string) *node {
	return &node{kind: kindScalar, goType: t, schema: apiextv1.JSONSchemaProps{Type: typ, Format: format}}
}

// lookup находит поле по указателю
func (b *builder) lookup(field any) (*fieldRef, error) {
	pv := reflect.ValueOf(field)
	if pv.Kind() != reflect.Pointer || pv.IsNil() {
		return nil, newError(FieldNotFound, "expected a pointer to a field of %s, got %T", b.rootName, field)
	}
	ref, ok := b.fields[fieldKey{pv.Pointer(), pv.Type().Elem()}]
	if !ok {
		return nil, newError(FieldNotFound, "%T is not a pointer to a field of %s", field, b.rootName)
	}
	if ref.inline {
		return nil, newError(FieldNotFound, "%s is an inline field, describe its fields instead", ref.goName)
	}
	return ref, nil
}

// field находит поле и проверяет, что правило вида want к нему применимо
func (b *builder) field(field any, want string, pos string) (*fieldRef, *node) {
	ref, err := b.lookup(field)
	if err != nil {
		b.add(pos, err)
		return nil, &node{kind: kindFixed}
	}
	if err := checkKind(ref.node, want); err != nil {
		b.add(pos, wrap(err, "%s", ref.goName))
		return nil, &node{kind: kindFixed}
	}
	return ref, ref.node
}

func (b *builder) add(pos string, err error) {
	b.errs = append(b.errs, wrap(err, "%s", pos))
}

// checkKind сверяет вид правила с JSON-видом поля
// Для типов со своим JSON-представлением Go-вид не совпадает с JSON: metav1.Time в Go —
// структура, а в JSON — строка. Поэтому для готовых схем вид берётся из схемы, а для
// ссылок на такие типы проверка пропускается: схема станет известна только при раскрытии
func checkKind(n *node, want string) error {
	if want == "any" {
		return nil
	}
	var ok bool
	switch {
	case n.kind == kindFixed && n.goType != nil && ownJSON(n.goType):
		ok = schemaKind(n.schema, want)
	case n.kind == kindRef && ownJSON(n.goType):
		ok = true
	default:
		ok = goKind(n, want)
	}
	if !ok {
		return newError(RuleKindMismatch, "%s rule is not applicable to %s", want, n.goType)
	}
	return nil
}

// ownJSON сообщает, что JSON-представление типа не выводится из его Go-вида
func ownJSON(t reflect.Type) bool {
	if _, ok := knownTypes[t]; ok {
		return true
	}
	pt := reflect.PointerTo(t)
	if pt.Implements(reflect.TypeFor[openAPISchemaTyper]()) {
		return true
	}
	return pt.Implements(marshalerType) || pt.Implements(textType)
}

// schemaKind сверяет вид правила с типом готовой схемы
func schemaKind(s apiextv1.JSONSchemaProps, want string) bool {
	switch want {
	case "string":
		return s.Type == "string" || s.XIntOrString
	case "integer":
		return s.Type == "integer" || s.XIntOrString
	case "number":
		return s.Type == "number"
	case "boolean":
		return s.Type == "boolean"
	case "object":
		return s.Type == "object"
	}
	return false
}

// goKind сверяет вид правила с Go-видом поля
func goKind(n *node, want string) bool {
	k := n.goType.Kind()
	switch want {
	case "string":
		return k == reflect.String
	case "integer":
		switch k {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return true
		}
	case "number":
		return k == reflect.Float32 || k == reflect.Float64
	case "boolean":
		return k == reflect.Bool
	case "array":
		return n.kind == kindArray
	case "map":
		return n.kind == kindMap
	case "object":
		return k == reflect.Struct && (n.kind == kindObject || n.kind == kindRef)
	}
	return false
}

// caller возвращает место вызова метода билдера в коде пользователя
func caller() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

type jsonOpts struct {
	omitempty bool
	omitzero  bool
	inline    bool
	dash      bool
}

func parseJSON(f reflect.StructField) (string, jsonOpts) {
	tag, ok := f.Tag.Lookup("json")
	if !ok {
		return "", jsonOpts{}
	}
	parts := strings.Split(tag, ",")
	opts := jsonOpts{dash: parts[0] == "-" && len(parts) > 1}
	for _, o := range parts[1:] {
		switch o {
		case "omitempty":
			opts.omitempty = true
		case "omitzero":
			opts.omitzero = true
		case "inline":
			opts.inline = true
		}
	}
	return parts[0], opts
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
