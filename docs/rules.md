# Описание схемы в коде

Как описать ограничения полей: длины, диапазоны, шаблоны, обязательность,
пресеты, CEL. Читайте, когда пишете `CRD()` для своего типа.

Откуда берётся схема поля, если правила нет, — в
[schema-sources.md](schema-sources.md).

## Пример

```go
type UserSpec struct {
	ClientID string   `json:"clientId"`
	TTL      int      `json:"ttl,omitempty"`
	Keys     []string `json:"keys,omitzero"`
	Address  Address  `json:"address"`
	Backup   *Address `json:"backup,omitempty"`
}

func (s *UserSpec) CRD(b crd.Builder) error {
	b.Self().Description("пользователь")
	b.K8sName(&s.ClientID).MaxLength(64).Description("идентификатор клиента")
	b.DurationSeconds(&s.TTL, 60, 3600).Default(900).Required()
	b.Array(&s.Keys).MinItems(1).Items(crd.HTTPSURL())
	b.String(&s.Address.City).Pattern("^[A-Z]").Optional()
	b.String(&s.Backup.Zip).Required()
	return nil
}
```

Метод `CRD` делает тип `crd.Describer`. Схема получается через
`crd.SchemaFor[UserSpec]()` или при сборке CRD, `CRD` вызывается один раз.

## Поля передаются указателем

Поле указывается адресом: `&s.TTL`. Переименование поля ловит компилятор,
опечатка невозможна.

`CRD` вызывается на экземпляре, который создаёт сам пакет: в нём можно брать
адреса полей, но не читать их значения.

Что адресуется:

- поля самой структуры и вложенных структур: `&s.Address.City`;
- поля структур за указателем: `&s.Backup.Zip` — память под нулевой указатель
  выделяется до вызова `CRD`;
- поля встроенных структур, как в `encoding/json`, в том числе без тега:
  `&s.Owner` для встроенной `Common`.

Элементы срезов и значения мап в памяти структуры не лежат и указателем не
адресуются. Для них есть `Items(правило)` и `Values(правило)` или описание на
типе элемента.

## Обязательность — свойство позиции

По умолчанию поле обязательно, если в json-теге нет `omitempty` или `omitzero`.
`Required()` и `Optional()` меняют это в родительском объекте и только в этой
позиции: `Optional()` для `address.city` не затрагивает `backup.city` того же
типа.

## Вид правила

Вид правила сверяется с JSON-видом поля сразу при вызове:

- к полю `string` подходит `String`, к `int` — `Integer`;
- к `metav1.Time` подходит `String`: в JSON это строка;
- к `intstr.IntOrString` — и `String`, и `Integer`.

Ошибки копятся и возвращаются вместе, с местом вызова:

```
crd: badKinds.CRD:
  user.go:14: badKinds.N: string rule is not applicable to int
  user.go:15: *string is not a pointer to a field of badKinds
```

## Методы Builder

| Метод | Для чего |
|---|---|
| `String`, `Integer`, `Number`, `Bool` | скаляры |
| `Array`, `Map`, `Object` | срезы, мапы, структуры |
| `Raw` | произвольный JSON (`x-kubernetes-preserve-unknown-fields`) |
| `IntOrString` | число или строка |
| `Use(&s.F, правило)` | заменить схему поля готовым правилом целиком |
| `Self()` | сам описываемый объект |
| `K8sName`, `HTTPSURL`, `NonEmptyString`, `Port`, `Percent`, `DurationSeconds` | пресеты |

У каждого правила есть `Description`, `Title`, `Nullable`, `Example`,
`XValidation` (CEL), `Required`, `Optional`. Кроме того:

- у строк — `MinLength`, `MaxLength`, `Pattern`, `Format`, `Enum`, `Default`;
- у чисел — `Minimum`, `Maximum`, `ExclusiveMinimum`, `ExclusiveMaximum`,
  `MultipleOf`, `Format`, `Enum`, `Default`;
- у массивов — `MinItems`, `MaxItems`, `UniqueItems`, `Items`, `ListType`,
  `ListMapKeys`;
- у мап — `MinProperties`, `MaxProperties`, `Values`, `MapType`;
- у объектов — `MinProperties`, `MaxProperties`, `MapType`,
  `PreserveUnknownFields`, `EmbeddedResource`;
- у `Raw` — `EmbeddedResource`.

### Комбинаторы

У строк и чисел есть `OneOf`, `AnyOf`, `AllOf`, `Not`, у объектов — `OneOf` и
`AnyOf`. Ветки не несут тип: так требует структурная схема CRD.

```go
ref := b.String(&s.Ref)
ref.OneOf().MaxLength(8).Pattern(`^[a-z]+$`)
ref.OneOf().Pattern(`^[0-9a-f]{32}$`)

solver := b.Object(&s.Solver)
solver.OneOf().RequiredFields("http01")
solver.OneOf().RequiredFields("dns01")
```

### CEL

```go
b.Self().XValidation("self.minReplicas <= self.maxReplicas", "minReplicas must not exceed maxReplicas")
```

### Неизвестные поля

Чтобы объект сохранял поля, которых нет в схеме, используйте
`PreserveUnknownFields()`. Не `additionalProperties: true`: при нём API-сервер
сохраняет незнакомые ключи, но вычищает их вложенное содержимое.
`additionalItems` API-сервер не поддерживает.

## Правила-значения

Правило без поля нужно для `Items`, `Values`, `Use` и для `Ruler`:

```go
crd.NewString(), crd.NewInteger(), crd.NewNumber(), crd.NewBool()
crd.NewArray(элементы), crd.NewMap(значения), crd.NewRaw(), crd.NewIntOrString()
crd.K8sName(), crd.HTTPSURL(), crd.NonEmptyString(), crd.Port(), crd.Percent(), crd.DurationSeconds(lo, hi)
```

У правил-значений нет `Required` и `Optional`: `crd.NewString().Required()` не
компилируется. Обязательность относится к позиции в объекте, а не к значению.

Чтобы сослаться на тип как на правило-значение, есть `crd.Of[T]()`. Например,
мапа, значение которой — непустой срез структур:

```go
b.Map(&s.Tags).Values(crd.NewArray(crd.Of[Tag]()).MinItems(1))
```

## Тип-значение: Ruler

Тип без полей (или со своим JSON-представлением) описывает себя методом `Rule`:

```go
type ClientID string

func (ClientID) Rule() (crd.Rule, error) {
	return crd.K8sName().MaxLength(64), nil
}
```

Схема `ClientID` используется во всех полях этого типа, правила поля
накладываются поверх:

```go
b.String(&s.Owner).Description("владелец") // pattern и maxLength из Rule, описание — поля
```

## Форматы

Пакет `crd.tools/crd/formats` содержит форматы OpenAPI, которые
проверяет API-сервер: `formats.DateTime`, `formats.Email`, `formats.UUID`,
`formats.Int64` и другие.

```go
b.String(&s.Created).Format(formats.DateTime)
```

Строковый литерал тоже подходит: список форматов открыт, неизвестный формат
API-сервер игнорирует.

## Смотрите также

- [schema-sources.md](schema-sources.md) — откуда берётся схема и что важнее;
- [crd.md](crd.md) — сборка CRD из описанных типов;
- [errors.md](errors.md) — коды ошибок;
- исполняемые примеры `Example_*` в `example*_test.go` — каждый печатает
  получившуюся схему.
