package crd

import (
	"crd.tools/crd/formats"
)

// Правила существуют в двух видах с одинаковыми ограничениями:
//   - значение (String, Integer, ...) описывает только значение, его возвращают NewString и пресеты;
//   - поле (StringField, IntegerField, ...) дополнительно знает позицию в родительском объекте,
//     его возвращают методы Builder, и только у него есть Required и Optional
//
// Ограничения описаны один раз в интерфейсах вида StringOf[S], где S — интерфейс, который
// возвращают методы для продолжения цепочки. Значение и поле встраивают их со своим S

// CommonOf это свойства, общие для правил всех видов
type CommonOf[S any] interface {
	// Description задаёт описание
	Description(s string) S

	// Title задаёт заголовок
	Title(s string) S

	// Nullable разрешает null
	Nullable() S

	// Example задаёт пример значения
	Example(v any) S

	// XValidation добавляет CEL-правило x-kubernetes-validations
	XValidation(rule, message string) S
}

// FieldOf это настройки позиции поля в родительском объекте
type FieldOf[S any] interface {
	// Required делает поле обязательным в родительском объекте
	Required() S

	// Optional делает поле необязательным в родительском объекте
	Optional() S
}

// StringOf это ограничения строки
type StringOf[S any] interface {
	CommonOf[S]

	// MinLength задаёт минимальную длину строки
	MinLength(n int64) S

	// MaxLength задаёт максимальную длину строки
	MaxLength(n int64) S

	// Pattern задаёт регулярное выражение ECMA-262
	Pattern(p string) S

	// Format задаёт формат значения, см. пакет formats
	Format(f formats.Format) S

	// Enum задаёт допустимые значения
	Enum(values ...string) S

	// Default задаёт значение по умолчанию
	Default(v string) S

	// OneOf добавляет ветку, из всех веток должна подойти ровно одна
	OneOf() StringBranch

	// AnyOf добавляет ветку, из всех веток должна подойти хотя бы одна
	AnyOf() StringBranch

	// AllOf добавляет ветку, которой значение должно соответствовать
	AllOf() StringBranch

	// Not задаёт ветку, которой значение соответствовать не должно
	Not() StringBranch
}

// IntegerOf это ограничения целого числа
type IntegerOf[S any] interface {
	CommonOf[S]

	// Minimum задаёт минимальное значение
	Minimum(n int64) S

	// Maximum задаёт максимальное значение
	Maximum(n int64) S

	// ExclusiveMinimum исключает минимум из допустимых значений
	ExclusiveMinimum() S

	// ExclusiveMaximum исключает максимум из допустимых значений
	ExclusiveMaximum() S

	// MultipleOf требует кратности значения
	MultipleOf(n int64) S

	// Format задаёт формат значения, см. пакет formats
	Format(f formats.Format) S

	// Enum задаёт допустимые значения
	Enum(values ...int64) S

	// Default задаёт значение по умолчанию
	Default(v int64) S

	// OneOf добавляет ветку, из всех веток должна подойти ровно одна
	OneOf() IntegerBranch

	// AnyOf добавляет ветку, из всех веток должна подойти хотя бы одна
	AnyOf() IntegerBranch

	// AllOf добавляет ветку, которой значение должно соответствовать
	AllOf() IntegerBranch

	// Not задаёт ветку, которой значение соответствовать не должно
	Not() IntegerBranch
}

// NumberOf это ограничения числа с плавающей точкой
type NumberOf[S any] interface {
	CommonOf[S]

	// Minimum задаёт минимальное значение
	Minimum(n float64) S

	// Maximum задаёт максимальное значение
	Maximum(n float64) S

	// ExclusiveMinimum исключает минимум из допустимых значений
	ExclusiveMinimum() S

	// ExclusiveMaximum исключает максимум из допустимых значений
	ExclusiveMaximum() S

	// MultipleOf требует кратности значения
	MultipleOf(n float64) S

	// Format задаёт формат значения, см. пакет formats
	Format(f formats.Format) S

	// Enum задаёт допустимые значения
	Enum(values ...float64) S

	// Default задаёт значение по умолчанию
	Default(v float64) S

	// OneOf добавляет ветку, из всех веток должна подойти ровно одна
	OneOf() NumberBranch

	// AnyOf добавляет ветку, из всех веток должна подойти хотя бы одна
	AnyOf() NumberBranch

	// AllOf добавляет ветку, которой значение должно соответствовать
	AllOf() NumberBranch

	// Not задаёт ветку, которой значение соответствовать не должно
	Not() NumberBranch
}

// BoolOf это ограничения булева значения
type BoolOf[S any] interface {
	CommonOf[S]

	// Default задаёт значение по умолчанию
	Default(v bool) S
}

// ArrayOf это ограничения массива, схема элемента берётся из Go-типа
type ArrayOf[S any] interface {
	CommonOf[S]

	// MinItems задаёт минимальное число элементов
	MinItems(n int64) S

	// MaxItems задаёт максимальное число элементов
	MaxItems(n int64) S

	// UniqueItems требует уникальности элементов
	UniqueItems() S

	// Items заменяет схему элемента готовым правилом
	Items(rule Rule) S

	// ListType задаёт x-kubernetes-list-type: atomic, set или map
	ListType(t string) S

	// ListMapKeys задаёт ключи элементов для x-kubernetes-list-type: map
	ListMapKeys(keys ...string) S
}

// MapOf это ограничения мапы, схема значения берётся из Go-типа
type MapOf[S any] interface {
	CommonOf[S]

	// MinProperties задаёт минимальное число ключей
	MinProperties(n int64) S

	// MaxProperties задаёт максимальное число ключей
	MaxProperties(n int64) S

	// Values заменяет схему значения готовым правилом
	Values(rule Rule) S

	// MapType задаёт x-kubernetes-map-type: granular или atomic
	MapType(t string) S
}

// ObjectOf это ограничения объекта, его поля берутся из Go-типа
type ObjectOf[S any] interface {
	CommonOf[S]

	// MinProperties задаёт минимальное число полей
	MinProperties(n int64) S

	// MaxProperties задаёт максимальное число полей
	MaxProperties(n int64) S

	// MapType задаёт x-kubernetes-map-type: granular или atomic
	MapType(t string) S

	// PreserveUnknownFields сохраняет поля, которых нет в схеме (x-kubernetes-preserve-unknown-fields)
	// Без него API-сервер удаляет незнакомые поля при записи
	PreserveUnknownFields() S

	// EmbeddedResource помечает объект как вложенный ресурс Kubernetes
	// (x-kubernetes-embedded-resource): в нём должны быть apiVersion, kind и metadata
	EmbeddedResource() S

	// OneOf добавляет ветку, из всех веток должна подойти ровно одна
	OneOf() ObjectBranch

	// AnyOf добавляет ветку, из всех веток должна подойти хотя бы одна
	AnyOf() ObjectBranch
}

// RawOf это свойства произвольного JSON
type RawOf[S any] interface {
	CommonOf[S]

	// EmbeddedResource помечает значение как вложенный ресурс Kubernetes
	// (x-kubernetes-embedded-resource): объект с apiVersion, kind и metadata, остальное произвольно
	EmbeddedResource() S
}

// String это правило-значение для строки
type String interface {
	Rule
	StringOf[String]
}

// StringField настраивает строковое поле
type StringField interface {
	StringOf[StringField]
	FieldOf[StringField]
}

// Integer это правило-значение для целого числа
type Integer interface {
	Rule
	IntegerOf[Integer]
}

// IntegerField настраивает целочисленное поле
type IntegerField interface {
	IntegerOf[IntegerField]
	FieldOf[IntegerField]
}

// Number это правило-значение для числа с плавающей точкой
type Number interface {
	Rule
	NumberOf[Number]
}

// NumberField настраивает поле с плавающей точкой
type NumberField interface {
	NumberOf[NumberField]
	FieldOf[NumberField]
}

// Bool это правило-значение для булева значения
type Bool interface {
	Rule
	BoolOf[Bool]
}

// BoolField настраивает булево поле
type BoolField interface {
	BoolOf[BoolField]
	FieldOf[BoolField]
}

// Array это правило-значение для массива
type Array interface {
	Rule
	ArrayOf[Array]
}

// ArrayField настраивает поле-массив
type ArrayField interface {
	ArrayOf[ArrayField]
	FieldOf[ArrayField]
}

// Map это правило-значение для мапы
type Map interface {
	Rule
	MapOf[Map]
}

// MapField настраивает поле-мапу
type MapField interface {
	MapOf[MapField]
	FieldOf[MapField]
}

// Object настраивает объект целиком, его возвращает Builder.Self
type Object interface {
	Rule
	ObjectOf[Object]
}

// ObjectField настраивает поле-структуру
type ObjectField interface {
	ObjectOf[ObjectField]
	FieldOf[ObjectField]
}

// Raw это правило-значение для произвольного JSON (x-kubernetes-preserve-unknown-fields)
type Raw interface {
	Rule
	RawOf[Raw]
}

// RawField настраивает поле с произвольным JSON
type RawField interface {
	RawOf[RawField]
	FieldOf[RawField]
}

// IntOrString это правило-значение для целого числа или строки
type IntOrString interface {
	Rule
	CommonOf[IntOrString]
}

// IntOrStringField настраивает поле, которое может быть целым числом или строкой
type IntOrStringField interface {
	CommonOf[IntOrStringField]
	FieldOf[IntOrStringField]
}

// Проверки на этапе компиляции, что каждая реализация подходит обоим интерфейсам своего вида
var (
	_ String           = stringRule[String]{}
	_ StringField      = stringRule[StringField]{}
	_ Integer          = integerRule[Integer]{}
	_ IntegerField     = integerRule[IntegerField]{}
	_ Number           = numberRule[Number]{}
	_ NumberField      = numberRule[NumberField]{}
	_ Bool             = boolRule[Bool]{}
	_ BoolField        = boolRule[BoolField]{}
	_ Array            = arrayRule[Array]{}
	_ ArrayField       = arrayRule[ArrayField]{}
	_ Map              = mapRule[Map]{}
	_ MapField         = mapRule[MapField]{}
	_ Object           = objectRule[Object]{}
	_ ObjectField      = objectRule[ObjectField]{}
	_ Raw              = rawRule[Raw]{}
	_ RawField         = rawRule[RawField]{}
	_ IntOrString      = commonRule[IntOrString]{}
	_ IntOrStringField = commonRule[IntOrStringField]{}
)

// rule это общая часть реализации правила, self — готовое правило нужного интерфейса
type rule[S any] struct {
	*base
	self S
}

// newRule создаёт правило вида K и приводит его к интерфейсу S один раз при создании
func newRule[S any, K any](b *base, kind func(*rule[S]) K) S {
	r := &rule[S]{base: b}
	r.self = any(kind(r)).(S)
	return r.self
}

func (r *rule[S]) Description(s string) S { r.n.schema.Description = s; return r.self }
func (r *rule[S]) Title(s string) S       { r.n.schema.Title = s; return r.self }
func (r *rule[S]) Nullable() S            { r.n.schema.Nullable = true; return r.self }
func (r *rule[S]) Example(v any) S        { r.example(v); return r.self }
func (r *rule[S]) Required() S            { r.setRequired(true); return r.self }
func (r *rule[S]) Optional() S            { r.setRequired(false); return r.self }
func (r *rule[S]) XValidation(expr, message string) S {
	r.xValidation(expr, message)
	return r.self
}

// commonRule это правило только с общими свойствами: Raw и IntOrString
type commonRule[S any] struct{ *rule[S] }

// ── строка ──

type stringRule[S any] struct{ *rule[S] }

func newString[S any](b *base) S {
	return newRule(b, func(r *rule[S]) stringRule[S] { return stringRule[S]{r} })
}

func (r stringRule[S]) MinLength(n int64) S       { r.n.schema.MinLength = &n; return r.self }
func (r stringRule[S]) MaxLength(n int64) S       { r.n.schema.MaxLength = &n; return r.self }
func (r stringRule[S]) Pattern(p string) S        { r.n.schema.Pattern = p; return r.self }
func (r stringRule[S]) Format(f formats.Format) S { r.n.schema.Format = f.String(); return r.self }
func (r stringRule[S]) Enum(values ...string) S   { addEnum(r.base, values); return r.self }
func (r stringRule[S]) Default(v string) S        { r.setDefault(v); return r.self }
func (r stringRule[S]) OneOf() StringBranch       { return stringBranch{branch(&r.n.schema.OneOf)} }
func (r stringRule[S]) AnyOf() StringBranch       { return stringBranch{branch(&r.n.schema.AnyOf)} }
func (r stringRule[S]) AllOf() StringBranch       { return stringBranch{branch(&r.n.schema.AllOf)} }
func (r stringRule[S]) Not() StringBranch         { return stringBranch{not(&r.n.schema)} }

// ── целое число ──

type integerRule[S any] struct{ *rule[S] }

func newInteger[S any](b *base) S {
	return newRule(b, func(r *rule[S]) integerRule[S] { return integerRule[S]{r} })
}

func (r integerRule[S]) Minimum(n int64) S         { r.n.schema.Minimum = ptr(float64(n)); return r.self }
func (r integerRule[S]) Maximum(n int64) S         { r.n.schema.Maximum = ptr(float64(n)); return r.self }
func (r integerRule[S]) ExclusiveMinimum() S       { r.n.schema.ExclusiveMinimum = true; return r.self }
func (r integerRule[S]) ExclusiveMaximum() S       { r.n.schema.ExclusiveMaximum = true; return r.self }
func (r integerRule[S]) MultipleOf(n int64) S      { r.n.schema.MultipleOf = ptr(float64(n)); return r.self }
func (r integerRule[S]) Format(f formats.Format) S { r.n.schema.Format = f.String(); return r.self }
func (r integerRule[S]) Enum(values ...int64) S    { addEnum(r.base, values); return r.self }
func (r integerRule[S]) Default(v int64) S         { r.setDefault(v); return r.self }
func (r integerRule[S]) OneOf() IntegerBranch      { return integerBranch{branch(&r.n.schema.OneOf)} }
func (r integerRule[S]) AnyOf() IntegerBranch      { return integerBranch{branch(&r.n.schema.AnyOf)} }
func (r integerRule[S]) AllOf() IntegerBranch      { return integerBranch{branch(&r.n.schema.AllOf)} }
func (r integerRule[S]) Not() IntegerBranch        { return integerBranch{not(&r.n.schema)} }

// ── число с плавающей точкой ──

type numberRule[S any] struct{ *rule[S] }

func newNumber[S any](b *base) S {
	return newRule(b, func(r *rule[S]) numberRule[S] { return numberRule[S]{r} })
}

func (r numberRule[S]) Minimum(n float64) S       { r.n.schema.Minimum = &n; return r.self }
func (r numberRule[S]) Maximum(n float64) S       { r.n.schema.Maximum = &n; return r.self }
func (r numberRule[S]) ExclusiveMinimum() S       { r.n.schema.ExclusiveMinimum = true; return r.self }
func (r numberRule[S]) ExclusiveMaximum() S       { r.n.schema.ExclusiveMaximum = true; return r.self }
func (r numberRule[S]) MultipleOf(n float64) S    { r.n.schema.MultipleOf = &n; return r.self }
func (r numberRule[S]) Format(f formats.Format) S { r.n.schema.Format = f.String(); return r.self }
func (r numberRule[S]) Enum(values ...float64) S  { addEnum(r.base, values); return r.self }
func (r numberRule[S]) Default(v float64) S       { r.setDefault(v); return r.self }
func (r numberRule[S]) OneOf() NumberBranch       { return numberBranch{branch(&r.n.schema.OneOf)} }
func (r numberRule[S]) AnyOf() NumberBranch       { return numberBranch{branch(&r.n.schema.AnyOf)} }
func (r numberRule[S]) AllOf() NumberBranch       { return numberBranch{branch(&r.n.schema.AllOf)} }
func (r numberRule[S]) Not() NumberBranch         { return numberBranch{not(&r.n.schema)} }

// ── булево значение ──

type boolRule[S any] struct{ *rule[S] }

func newBool[S any](b *base) S {
	return newRule(b, func(r *rule[S]) boolRule[S] { return boolRule[S]{r} })
}

func (r boolRule[S]) Default(v bool) S { r.setDefault(v); return r.self }

// ── массив ──

type arrayRule[S any] struct{ *rule[S] }

func newArray[S any](b *base) S {
	return newRule(b, func(r *rule[S]) arrayRule[S] { return arrayRule[S]{r} })
}

func (r arrayRule[S]) MinItems(n int64) S { r.n.schema.MinItems = &n; return r.self }
func (r arrayRule[S]) MaxItems(n int64) S { r.n.schema.MaxItems = &n; return r.self }
func (r arrayRule[S]) UniqueItems() S     { r.n.schema.UniqueItems = true; return r.self }
func (r arrayRule[S]) ListType(t string) S {
	r.n.schema.XListType = &t
	return r.self
}
func (r arrayRule[S]) ListMapKeys(keys ...string) S {
	r.n.schema.XListMapKeys = append(r.n.schema.XListMapKeys, keys...)
	return r.self
}
func (r arrayRule[S]) Items(rule Rule) S {
	r.replaceWith(rule, func(n *node) { r.n.items = n })
	return r.self
}

// ── мапа ──

type mapRule[S any] struct{ *rule[S] }

func newMap[S any](b *base) S {
	return newRule(b, func(r *rule[S]) mapRule[S] { return mapRule[S]{r} })
}

func (r mapRule[S]) MinProperties(n int64) S { r.n.schema.MinProperties = &n; return r.self }
func (r mapRule[S]) MaxProperties(n int64) S { r.n.schema.MaxProperties = &n; return r.self }
func (r mapRule[S]) MapType(t string) S      { r.n.schema.XMapType = &t; return r.self }
func (r mapRule[S]) Values(rule Rule) S {
	r.replaceWith(rule, func(n *node) { r.n.values = n })
	return r.self
}

// ── объект ──

type objectRule[S any] struct{ *rule[S] }

func newObject[S any](b *base) S {
	return newRule(b, func(r *rule[S]) objectRule[S] { return objectRule[S]{r} })
}

func (r objectRule[S]) MinProperties(n int64) S { r.n.schema.MinProperties = &n; return r.self }
func (r objectRule[S]) MaxProperties(n int64) S { r.n.schema.MaxProperties = &n; return r.self }
func (r objectRule[S]) MapType(t string) S      { r.n.schema.XMapType = &t; return r.self }
func (r objectRule[S]) PreserveUnknownFields() S {
	r.n.schema.XPreserveUnknownFields = ptr(true)
	return r.self
}
func (r objectRule[S]) EmbeddedResource() S {
	r.n.schema.XEmbeddedResource = true
	return r.self
}
func (r objectRule[S]) OneOf() ObjectBranch { return objectBranch{branch(&r.n.schema.OneOf)} }
func (r objectRule[S]) AnyOf() ObjectBranch { return objectBranch{branch(&r.n.schema.AnyOf)} }

// ── произвольный JSON ──

type rawRule[S any] struct{ *rule[S] }

func newRaw[S any](b *base) S {
	return newRule(b, func(r *rule[S]) rawRule[S] { return rawRule[S]{r} })
}

// EmbeddedResource требует у значения тип object, как того хочет API-сервер
func (r rawRule[S]) EmbeddedResource() S {
	r.n.schema.Type = "object"
	r.n.schema.XEmbeddedResource = true
	return r.self
}

// ── только общие свойства ──

func newCommon[S any](b *base) S {
	return newRule(b, func(r *rule[S]) commonRule[S] { return commonRule[S]{r} })
}
