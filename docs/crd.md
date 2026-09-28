# Сборка CRD

Как из описанного типа получить CustomResourceDefinition: группа, версии,
подресурсы, колонки kubectl, конверсия, проверка. Читайте, когда объявляете
ресурс.

Пример с подресурсами status и scale по шагам — в
[status-and-scale.md](status-and-scale.md).

## Пример

```go
package resources

var App = crd.New[apps.App]().
	Group("example.com").Kind("App").Plural("apps").
	Version("v1", true, true, crd.Status())

func init() {
	crd.Add(App)
}
```

Манифест:

```go
err := App.Encode(os.Stdout)
```

## Тип ресурса

Схема берётся из типа `T` так же, как в `SchemaFor`: из `CRD()`, из снимков или
выводом из Go-типа.

```go
type App struct {
	metav1.TypeMeta   `json:",inline"`          // apiVersion и kind
	metav1.ObjectMeta `json:"metadata,omitempty"` // metadata

	Spec   AppSpec   `json:"spec"`
	Status AppStatus `json:"status,omitempty"`
}
```

- Поля верхнего уровня могут называться как угодно: `spec`, `data`, что-то своё.
- Схему строят JSON-имена, поэтому нужны json-теги: без тега поле `Status`
  называлось бы `Status`, а не `status`.
- Если `TypeMeta` и `ObjectMeta` в типе нет, корень всё равно дополняется
  `apiVersion`, `kind` и `metadata`.
- `metadata` описывается как `object` без полей: его проверяет сам API-сервер.

Всё, что относится к ресурсу, а не к схеме, из типа не берётся: группа, вид,
область, короткие имена и подресурсы задаются только в `crd.New`. Маркеры
`+kubebuilder:resource` и методы `runtime.Object` не учитываются.

## Настройки ресурса

| Метод | По умолчанию |
|---|---|
| `Group`, `Kind`, `Plural` | обязательны |
| `Singular` | `Kind` в нижнем регистре |
| `Scope` | `Namespaced` |
| `ShortNames`, `Categories` | нет |
| `Annotation`, `Label` | нет |
| `ConversionWebhook(cfg, версии...)` | конверсия `None` |
| `Modifiers(m...)` | без модификаторов, см. [kubebuild.md](kubebuild.md#модификаторы) |

`ListKind` всегда `Kind` + `List`.

## Версии

```go
crd.New[App]().
	Group("example.com").Kind("App").Plural("apps").
	Version("v1", true, true,
		crd.Status(),
		crd.Scale(".spec.replicas", ".status.replicas", ".status.selector"),
		crd.PrinterColumn("Replicas", "integer", ".spec.replicas"),
	).
	Version("v1beta1", true, false,
		crd.VersionType[v1beta1.App](), // своя схема у старой версии
		crd.Deprecated("используйте v1"),
	)
```

`Version(имя, served, storage, опции...)`. Нужна ровно одна версия хранения.

| Опция | Что делает |
|---|---|
| `Status()` | подресурс status: статус пишется отдельно через `/status` |
| `Scale(spec, status, selector)` | подресурс scale для `kubectl scale` и HPA |
| `PrinterColumn(имя, тип, jsonPath)` | колонка в `kubectl get` |
| `PrinterColumnDef(...)` | колонка с описанием, форматом и приоритетом |
| `Deprecated(предупреждение)` | устаревшая версия |
| `VersionType[V]()` | схема версии из другого типа |

У CRD только два подресурса: `status` и `scale`. Подробно о них — в
[status-and-scale.md](status-and-scale.md).

## Определения в рантайме

Той же переменной пользуется приложение: для регистрации типа в
`runtime.Scheme`, подписки на события и выбора между `Update` и `UpdateStatus`.

```go
gvk := resources.App.GroupVersionKind("v1")
gvr := resources.App.GroupVersionResource("v1")
resources.App.Namespaced()
resources.App.HasStatus("v1")
```

Эти методы читают настройки определения и не вызывают `Build`.

`crd.Add` добавляет определение в реестр, `crd.List()` возвращает все
добавленные в порядке добавления. Повторный `Add` того же определения ничего не
делает. Инструмент находит `crd.Add` только внутри `init`.

## Сборка и проверка в коде

```go
obj, err := App.Build()           // *apiextv1.CustomResourceDefinition
data, err := crd.Marshal(obj)     // YAML-документ без status
err = App.Encode(os.Stdout)       // Build + Marshal

err = validation.Validate(ctx, obj) // проверка кодом API-сервера
```

Ошибки настройки накапливаются: первая запоминается, дальнейшие вызовы ничего
не делают, а `Err`, `Build` и `Encode` её возвращают.

`crd.tools/crd/validation` проверяет CRD так же, как API-сервер при
создании: структурность схем, подресурсы, колонки, конверсию, имена. Перед
проверкой применяются значения по умолчанию v1, как на сервере. Проверка
вынесена в отдельный пакет, потому что удваивает число зависимостей.

API-сервер проверяет не всё: например, путь `Scale` должен начинаться с `.spec`,
но его наличие в схеме не проверяется.

## Смотрите также

- [status-and-scale.md](status-and-scale.md) — ресурс со status и scale по шагам;
- [rules.md](rules.md) — описание схемы полей.
