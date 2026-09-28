package crd

import (
	"encoding/json"
	"reflect"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"crd.tools/crd/formats"
)

// Field это настройки позиции поля без смены его схемы, их возвращает Builder.Use
type Field interface {
	// Description задаёт описание
	Description(s string) Field

	// Required делает поле обязательным в родительском объекте
	Required() Field

	// Optional делает поле необязательным в родительском объекте
	Optional() Field
}

// base это общее состояние правила поля или правила-значения
type base struct {
	// b билдер, nil для правила-значения
	b *builder

	// pos место вызова в коде пользователя
	pos string

	// ref поле в родительском объекте, nil для правила-значения и для Self
	ref *fieldRef

	// n узел, который настраивается
	n *node

	// broken поле не найдено, ошибка уже записана, дальнейшие вызовы игнорируются
	broken bool

	// err первая ошибка правила-значения
	err error
}

func (b *builder) newBase(pos string, ref *fieldRef, n *node) *base {
	return &base{b: b, pos: pos, ref: ref, n: n, broken: ref == nil}
}

func newValue(n *node) *base {
	return &base{n: n}
}

// Schema возвращает схему правила
//
//	InvalidRule
func (r *base) Schema() (apiextv1.JSONSchemaProps, error) {
	if r.err != nil {
		return apiextv1.JSONSchemaProps{}, r.err
	}
	return r.n.render(), nil
}

func (r *base) fail(err error) {
	if r.b != nil {
		r.b.add(r.pos, err)
		return
	}
	if r.err == nil {
		r.err = err
	}
}

func (r *base) setRequired(v bool) {
	if r.broken || r.ref == nil {
		return
	}
	r.ref.parent.required[r.ref.name] = v
}

func (r *base) setDefault(v any) {
	d, err := jsonValue(v)
	if err != nil {
		r.fail(err)
		return
	}
	r.n.schema.Default = &d
}

func (r *base) example(v any) {
	e, err := jsonValue(v)
	if err != nil {
		r.fail(err)
		return
	}
	r.n.schema.Example = &e
}

func (r *base) xValidation(rule, message string) {
	r.n.schema.XValidations = append(r.n.schema.XValidations, apiextv1.ValidationRule{Rule: rule, Message: message})
}

func (r *base) replaceWith(rule Rule, set func(*node)) {
	if r.broken {
		return
	}
	if rule == nil {
		r.fail(newError(InvalidRule, "nil rule"))
		return
	}
	s, err := rule.Schema()
	if err != nil {
		r.fail(err)
		return
	}
	set(&node{kind: kindFixed, schema: s})
}

func addEnum[T any](r *base, values []T) {
	for _, v := range values {
		e, err := jsonValue(v)
		if err != nil {
			r.fail(err)
			return
		}
		r.n.schema.Enum = append(r.n.schema.Enum, e)
	}
}

func jsonValue(v any) (apiextv1.JSON, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return apiextv1.JSON{}, newError(InvalidRule, "encode value: %s", err)
	}
	return apiextv1.JSON{Raw: data}, nil
}

// ── позиция поля для Use ──

type fieldPosition struct{ *base }

func (r fieldPosition) Description(s string) Field { r.n.schema.Description = s; return r }
func (r fieldPosition) Required() Field            { r.setRequired(true); return r }
func (r fieldPosition) Optional() Field            { r.setRequired(false); return r }

// ── методы билдера ──

func (b *builder) Self() Object {
	return newObject[Object](&base{b: b, pos: caller(), n: b.root})
}

func (b *builder) String(field any) StringField   { return b.stringAt(field, caller()) }
func (b *builder) Integer(field any) IntegerField { return b.integerAt(field, caller()) }

func (b *builder) Number(field any) NumberField {
	pos := caller()
	ref, n := b.field(field, "number", pos)
	return newNumber[NumberField](b.newBase(pos, ref, n))
}

func (b *builder) Bool(field any) BoolField {
	pos := caller()
	ref, n := b.field(field, "boolean", pos)
	return newBool[BoolField](b.newBase(pos, ref, n))
}

func (b *builder) Array(field any) ArrayField {
	pos := caller()
	ref, n := b.field(field, "array", pos)
	return newArray[ArrayField](b.newBase(pos, ref, n))
}

func (b *builder) Map(field any) MapField {
	pos := caller()
	ref, n := b.field(field, "map", pos)
	return newMap[MapField](b.newBase(pos, ref, n))
}

func (b *builder) Object(field any) ObjectField {
	pos := caller()
	ref, n := b.field(field, "object", pos)
	return newObject[ObjectField](b.newBase(pos, ref, n))
}

func (b *builder) Raw(field any) RawField {
	pos := caller()
	ref, n := b.field(field, "any", pos)
	if ref != nil {
		n.replace(apiextv1.JSONSchemaProps{XPreserveUnknownFields: ptr(true)})
	}
	return newRaw[RawField](b.newBase(pos, ref, n))
}

func (b *builder) IntOrString(field any) IntOrStringField {
	pos := caller()
	ref, n := b.field(field, "any", pos)
	if ref != nil {
		n.replace(apiextv1.JSONSchemaProps{XIntOrString: true})
	}
	return newCommon[IntOrStringField](b.newBase(pos, ref, n))
}

func (b *builder) Use(field any, r Rule) Field {
	pos := caller()
	ref, n := b.field(field, "any", pos)
	f := fieldPosition{b.newBase(pos, ref, n)}
	if ref == nil {
		return f
	}
	if r == nil {
		b.add(pos, newError(InvalidRule, "Use: nil rule"))
		return f
	}
	s, err := r.Schema()
	if err != nil {
		b.add(pos, err)
		return f
	}
	n.replace(s)
	return f
}

func (b *builder) K8sName(field any) StringField {
	return b.stringAt(field, caller()).Pattern(k8sNamePattern)
}

func (b *builder) HTTPSURL(field any) StringField {
	return b.stringAt(field, caller()).Pattern(httpsPattern)
}

func (b *builder) NonEmptyString(field any) StringField {
	return b.stringAt(field, caller()).MinLength(1)
}

func (b *builder) Port(field any) IntegerField {
	return b.integerAt(field, caller()).Format(formats.Int32).Minimum(1).Maximum(65535)
}

func (b *builder) Percent(field any) IntegerField {
	return b.integerAt(field, caller()).Format(formats.Int32).Minimum(0).Maximum(100)
}

func (b *builder) DurationSeconds(field any, lo, hi int64) IntegerField {
	return b.integerAt(field, caller()).Minimum(lo).Maximum(hi)
}

func (b *builder) stringAt(field any, pos string) StringField {
	ref, n := b.field(field, "string", pos)
	return newString[StringField](b.newBase(pos, ref, n))
}

func (b *builder) integerAt(field any, pos string) IntegerField {
	ref, n := b.field(field, "integer", pos)
	return newInteger[IntegerField](b.newBase(pos, ref, n))
}

// ── правила-значения для Ruler, Use, Items и Values ──

const (
	k8sNamePattern = `^[a-z0-9-]+$`
	httpsPattern   = `^https://`
)

// NewString возвращает правило-значение для строки
func NewString() String {
	return newString[String](newValue(scalar(reflect.TypeFor[string](), "string", "")))
}

// NewInteger возвращает правило-значение для целого числа
func NewInteger() Integer {
	return newInteger[Integer](newValue(scalar(reflect.TypeFor[int64](), "integer", "")))
}

// NewNumber возвращает правило-значение для числа с плавающей точкой
func NewNumber() Number {
	return newNumber[Number](newValue(scalar(reflect.TypeFor[float64](), "number", "")))
}

// NewBool возвращает правило-значение для булева значения
func NewBool() Bool {
	return newBool[Bool](newValue(scalar(reflect.TypeFor[bool](), "boolean", "")))
}

// NewArray возвращает правило-значение для массива с элементами items
func NewArray(items Rule) Array {
	n := &node{kind: kindArray, schema: apiextv1.JSONSchemaProps{Type: "array"}, items: &node{kind: kindFixed}}
	return newArray[Array](newValue(n)).Items(items)
}

// NewMap возвращает правило-значение для мапы со значениями values
func NewMap(values Rule) Map {
	n := &node{kind: kindMap, schema: apiextv1.JSONSchemaProps{Type: "object"}, values: &node{kind: kindFixed}}
	return newMap[Map](newValue(n)).Values(values)
}

// NewRaw возвращает правило-значение для произвольного JSON
func NewRaw() Raw {
	return newRaw[Raw](newValue(&node{kind: kindFixed, schema: apiextv1.JSONSchemaProps{XPreserveUnknownFields: ptr(true)}}))
}

// NewIntOrString возвращает правило-значение для целого числа или строки
func NewIntOrString() IntOrString {
	return newCommon[IntOrString](newValue(&node{kind: kindFixed, schema: apiextv1.JSONSchemaProps{XIntOrString: true}}))
}

// K8sName это имя в стиле DNS: строчные буквы, цифры и дефис
func K8sName() String { return NewString().Pattern(k8sNamePattern) }

// HTTPSURL это строка, которая начинается с https://
func HTTPSURL() String { return NewString().Pattern(httpsPattern) }

// NonEmptyString это непустая строка
func NonEmptyString() String { return NewString().MinLength(1) }

// Port это TCP-порт 1..65535
func Port() Integer { return NewInteger().Format(formats.Int32).Minimum(1).Maximum(65535) }

// Percent это процент 0..100
func Percent() Integer { return NewInteger().Format(formats.Int32).Minimum(0).Maximum(100) }

// DurationSeconds это длительность в секундах в границах [lo, hi]
func DurationSeconds(lo, hi int64) Integer { return NewInteger().Minimum(lo).Maximum(hi) }
