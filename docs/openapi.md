# Документ OpenAPI для SDK

Как получить OpenAPI v3 по определениям CRD, где каждый именованный тип — отдельный
компонент со ссылками `$ref`. Читайте, когда по CRD нужно сгенерировать SDK для
других языков.

## Зачем, если OpenAPI есть у кластера

API-сервер публикует OpenAPI для CRD, но строит его из схемы CRD, а в ней `$ref`
запрещены: все вложенные объекты описаны на месте. Генераторы SDK называют их по
пути к полю, и получаются классы вроде `WorkerSpecSelectorMatchExpressionsInner`.

Мы строим документ из Go-типов, поэтому границы типов сохраняются: в SDK будут
`Worker`, `WorkerSpec`, `LabelSelector` — как в Go.

## Пример

```go
doc, err := crd.OpenAPI(crd.OpenAPIConfig{
	Title:    "workers",
	Version:  "v1",
	Prefixes: []crd.Prefix{{From: "k8s.io.apimachinery.pkg", To: "apimachinery"}},
}, workers.Resource)
data, err := doc.YAML()
```

Получится:

```yaml
openapi: 3.0.3
components:
  schemas:
    workers.Worker:
      properties:
        spec: {$ref: '#/components/schemas/workers.WorkerSpec'}
        status: {$ref: '#/components/schemas/workers.WorkerStatus'}
    workers.WorkerSpec:
      properties:
        replicas: {type: integer, format: int32, minimum: 0, maximum: 100}
        selector: {$ref: '#/components/schemas/apimachinery.apis.meta.v1.LabelSelector'}
    apimachinery.apis.meta.v1.LabelSelector:
      properties:
        matchExpressions:
          items: {$ref: '#/components/schemas/apimachinery.apis.meta.v1.LabelSelectorRequirement'}
```

## Имена компонентов

Имя строится из пути Go-пакета и имени типа, `/` заменяется на точку:

```
k8s.io/apimachinery/pkg/apis/meta/v1.LabelSelector
→ k8s.io.apimachinery.pkg.apis.meta.v1.LabelSelector
```

Каждый тип живёт ровно в одном компоненте — по адресу своего пакета. Поэтому
общая структура, которую используют несколько CRD, описана один раз, и все
ссылаются на неё.

### Замены префиксов

Полные имена длинные, их сокращают заменами начала имени:

| Замена | Было | Стало |
|---|---|---|
| `k8s.io.apimachinery.pkg` → `apimachinery` | `k8s.io.apimachinery.pkg.apis.meta.v1.LabelSelector` | `apimachinery.apis.meta.v1.LabelSelector` |
| `example.com.project` → (пусто) | `example.com.project.workers.WorkerSpec` | `workers.WorkerSpec` |

- Замена применяется по границе точки: `k8s.io.api` не заденет `k8s.io.apimachinery`.
- Если подходят несколько замен, выбирается самая длинная.
- Если после замен у двух типов получилось одно имя, это ошибка с указанием
  обоих: поправьте замены.

Замены задаются в `OpenAPIConfig.Prefixes`.

## Ссылки и свойства поля

- **Поле без своих правил** — просто ссылка: `{$ref: ...}`.
- **Поле со своим описанием или ограничениями** — ссылка в `allOf`, свойства
  рядом. В OpenAPI 3.0 всё, что лежит рядом с `$ref`, игнорируется, а `allOf` с
  одной ссылкой генераторы понимают как тип:

  ```yaml
  owner:
    description: владелец
    allOf:
    - $ref: '#/components/schemas/api.ClientID'
  ```

- **Поле, где правила родителя меняют вложенный тип** (например,
  `b.String(&s.Address.City).Optional()`), описывается на месте, без ссылки: общий
  компонент не должен нести правила одной позиции.
- **Известные типы** (`metav1.Time`, `resource.Quantity` и др.) остаются на месте,
  как в CRD. `metadata` — объект без полей.

Схемы берутся по тем же правилам, что и в `SchemaFor`
([schema-sources.md](schema-sources.md)): `CRD()`, `Rule()`, `RegisterSchema`,
снимки, вывод из Go-типа.

## Что пока не делается

- **Пути (paths)** — в документе только модели. Запросы SDK обычно делает через
  клиент Kubernetes; пути с list/get/watch и подресурсами можно добавить позже.
- **`x-kubernetes-int-or-string`** остаётся как есть; генераторы обычно делают из
  такого поля `any`.

## Смотрите также

- [crd.md](crd.md) — определения CRD.
