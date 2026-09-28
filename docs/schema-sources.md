# Откуда берётся схема

Какой источник схемы побеждает, когда их несколько: правило из кода, снимок
kubebuilder, встроенное знание о типе, вывод из Go-типа. Читайте, если схема
получилась не такой, как ожидали, или вы подключаете чужие типы.

## Два слоя

Схема поля складывается из **базовой схемы типа** поля и **правил поля** из
`CRD()` родителя поверх неё.

```go
type ClientID string

func (ClientID) Rule() (crd.Rule, error) { return crd.K8sName().MaxLength(64), nil }

func (s *Spec) CRD(b crd.Builder) error {
	b.String(&s.Owner).Description("владелец") // базовая схема — из Rule, описание — правило поля
	return nil
}
```

Правило поля действует только в этой позиции и не меняет схему типа.

## Базовая схема типа

Первое подходящее, сверху вниз:

| # | Источник | Как попадает в схему |
|---|---|---|
| 1 | метка `crd:"kubebuild"` на поле | `$ref` на тип, разрешается по реестру |
| 2 | `Describer`, `Ruler`, `RegisterSchema` | `$ref` на тип, тип регистрируется автоматически |
| 3 | таблица известных типов | схема подставляется как есть |
| 4 | методы kube-openapi (`OpenAPISchemaType` и др.) | схема строится по методам |
| 5 | `MarshalJSON`/`MarshalText` без описания | ошибка `UnsupportedType` |
| 6 | вывод из Go-типа | поля структуры, элементы, скаляры |

`$ref` разрешается по реестру, и в нём **схема из кода важнее снимка**, в том
числе внутри поддерева под меткой.

Для самого типа `SchemaFor`, `New` и `Of` идут в том же порядке: код, снимок,
встроенная схема, вывод из Go-типа.

## Примеры старшинства

**Код важнее снимка:**

```go
type Spec struct {
	Res legacy.Resources `json:"res" crd:"kubebuild"` // схема из снимка...
}

func init() {
	crd.RegisterSchema(legacy.ResourceList{}, custom) // ...но ResourceList внутри неё — эта
}
```

**Код важнее встроенной схемы:**

```go
func init() {
	crd.RegisterSchema(metav1.Time{}, apiextv1.JSONSchemaProps{Type: "string", Pattern: "^20"})
}
```

**Таблица важнее методов kube-openapi:** `resource.Quantity` описывает себя как
«строка или число», а из таблицы получает ещё и шаблон значения.

**Методы kube-openapi важнее `MarshalJSON`:** тип со своим JSON-представлением
годится, если описывает схему сам:

```go
type IP struct{ b [4]byte }

func (IP) MarshalJSON() ([]byte, error) { ... }
func (IP) OpenAPISchemaType() []string  { return []string{"string"} }
func (IP) OpenAPISchemaFormat() string  { return "ipv4" } // → type: string, format: ipv4
```

Значение, которое может быть и числом, и строкой (формат `int-or-string` или
`OpenAPIV3OneOfTypes` с числом и строкой), получает
`x-kubernetes-int-or-string`. Сочетания, которые CRD выразить не может, —
ошибка `UnsupportedType`.

**Без описания тип со своим JSON — ошибка**, а не угаданная схема: по полям
`time.Time` получился бы пустой объект вместо строки.

```go
type Opaque struct{ v string }

func (Opaque) MarshalJSON() ([]byte, error) { ... } // поле Opaque → UnsupportedType
```

## Известные типы

| Тип | Схема |
|---|---|
| `time.Time`, `metav1.Time`, `metav1.MicroTime` | `string`, `date-time` |
| `metav1.Duration` | `string` |
| `resource.Quantity` | число или строка с шаблоном |
| `intstr.IntOrString` | число или строка |
| `json.RawMessage`, `apiextv1.JSON` | произвольный JSON |
| `runtime.RawExtension` | произвольный объект |
| `metav1.ObjectMeta` | `object` без полей: `metadata` проверяет сам API-сервер |

`time.Duration` в таблице нет: своего `MarshalJSON` у него нет, в JSON это число
наносекунд, и вывод из Go-типа даёт верный `integer`.

## Как тип со своим JSON получает схему

| Способ | Когда подходит |
|---|---|
| `Rule()` на типе | свой тип |
| методы kube-openapi | свой тип, совместимость с экосистемой Kubernetes |
| `crd.RegisterSchema(T{}, схема)` | чужой тип, к которому не добавить методы |
| метка `crd:"kubebuild"` на поле | тип с kubebuilder-маркерами |
| таблица известных типов | стандартные типы, ничего делать не нужно |

`CRD()` (`Describer`) для такого типа не подходит: он описывает Go-поля, а они не
совпадают с JSON-представлением.

## Рантайм и кеш

| Функция | Что делает |
|---|---|
| `SchemaFor[T]()`, `New[T]().Build()` | при первом обращении обходят граф Go-типов, включая поля типов из снимков, и вызывают `CRD()`/`Rule()` найденных типов; дальше — из кеша |
| `Of[T]()` | тип регистрируется при первом разрешении схемы |
| `Register(v)` | всегда вызывает `CRD()`/`Rule()` заново и перезаписывает схему |
| `Schema(key)` | только кеш: по строке нельзя найти Go-тип |

`CRD()` и `Rule()` каждого типа вызываются один раз. Для запросов по ключу
(`Schema("pkg.Type")`) регистрируйте такие типы в `init` их пакета:

```go
func init() { crd.Register(ClientID("")) }
```

Схема из `RegisterSchema`, зарегистрированная раньше первого обращения к типу,
заменяет его `CRD()`/`Rule()`.

## Раскрытая схема

`SchemaFor` и `Schema` возвращают схему с раскрытыми `$ref` и слитым `allOf`, как
это делает controller-tools:

- документация берётся с поля;
- ограничения поля и типа сливаются;
- конфликтующие ограничения остаются в `allOf`, чтобы применились оба.

Все правила этого документа покрыты тестами в `priority_test.go`.

## Смотрите также

- [rules.md](rules.md) — правила полей;
- [kubebuild.md](kubebuild.md) — снимки из kubebuilder-маркеров.
