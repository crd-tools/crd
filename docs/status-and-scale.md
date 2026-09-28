# Ресурс с подресурсами status и scale

Пример ресурса `Worker`, у которого одновременно включены оба подресурса CRD:

- **status** — статус пишется отдельно от `spec`, через `/status`;
- **scale** — ресурс масштабируется через `kubectl scale` и HorizontalPodAutoscaler.

Шаги: [структуры](#1-структуры) → [определение CRD](#2-определение-crd) →
[регистрация](#3-регистрация-в-init) → [манифест](#4-манифест) →
[на что обратить внимание](#на-что-обратить-внимание) →
[как пользоваться](#как-пользоваться).

## 1. Структуры

```go
package workers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"crd.tools/crd"
)

// WorkerSpec это желаемое состояние
type WorkerSpec struct {
	Replicas int32                `json:"replicas"`
	Selector metav1.LabelSelector `json:"selector"`
	Image    string               `json:"image"`
}

func (s *WorkerSpec) CRD(b crd.Builder) error {
	b.Integer(&s.Replicas).Minimum(0).Maximum(100).Description("желаемое число реплик")
	b.String(&s.Image).MinLength(1)
	return nil
}

// WorkerStatus это фактическое состояние, его пишет контроллер
type WorkerStatus struct {
	Replicas int32  `json:"replicas,omitempty"`
	Selector string `json:"selector,omitempty"`
}

func (s *WorkerStatus) CRD(b crd.Builder) error {
	b.Integer(&s.Replicas).Minimum(0).Description("фактическое число реплик")
	b.String(&s.Selector).Description("селектор подов строкой, для HPA")
	return nil
}

// Worker это ресурс
type Worker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerSpec   `json:"spec"`
	Status WorkerStatus `json:"status,omitempty"`
}
```

## 2. Определение CRD

Подресурсы включаются опциями версии:

```go
// Resource это определение CRD, его же использует приложение
var Resource = crd.New[Worker]().
	Group("example.com").Kind("Worker").Plural("workers").
	Version("v1", true, true,
		crd.Status(),
		crd.Scale(".spec.replicas", ".status.replicas", ".status.selector"),
		crd.PrinterColumn("Desired", "integer", ".spec.replicas"),
		crd.PrinterColumn("Ready", "integer", ".status.replicas"),
	)
```

`crd.Scale` принимает три пути:

| Путь | Что это | Ограничение |
|---|---|---|
| `.spec.replicas` | желаемое число реплик | внутри `.spec` |
| `.status.replicas` | фактическое число реплик | внутри `.status` |
| `.status.selector` | селектор подов строкой | необязательный, `""` — без него |

## 3. Регистрация в init

```go
func init() {
	crd.Add(Resource)
}
```

## 4. Манифест

```go
obj, err := Resource.Build()
err = validation.Validate(ctx, obj) // проверка кодом API-сервера
data, err := crd.Marshal(obj)
```

В манифесте подресурсы и колонки выглядят так:

```yaml
subresources:
  scale:
    labelSelectorPath: .status.selector
    specReplicasPath: .spec.replicas
    statusReplicasPath: .status.replicas
  status: {}
additionalPrinterColumns:
- jsonPath: .spec.replicas
  name: Desired
  type: integer
- jsonPath: .status.replicas
  name: Ready
  type: integer
```

`status: {}` — не заглушка: у подресурса status нет настроек, само его наличие
включает поведение.

## На что обратить внимание

**Реплики — `int32`.** `/scale` работает со стандартным объектом
`autoscaling/v1 Scale`, где число реплик 32-битное. С `int64` в `spec` значение
больше 2³¹ не пройдёт через `/scale`.

**Селектор в `status` — строка, а не `LabelSelector`.** `labelSelectorPath`
должен указывать на строку вида `app=worker,tier=backend`, её ждёт HPA. В `spec`
селектор удобно держать структурой, а контроллер пишет в статус его строковую
форму, например через `metav1.FormatLabelSelector`. Если HPA по метрикам подов
не нужен, `labelSelectorPath` можно не задавать.

**Статус заполняет контроллер.** Пока `status.replicas` не записан,
`kubectl get` покажет пустую колонку `Ready`, а HPA не увидит текущее число
реплик.

**Поле статуса — `status`, в нижнем регистре, на верхнем уровне.** Подресурс
жёстко привязан к JSON-ключу `status`. Без json-тега поле называлось бы `Status`,
и подресурс его бы не увидел.

**`Status` без указателя, но с `omitempty`.** Поле необязательное, поэтому новый
объект без статуса проходит проверку.

## Как пользоваться

Из командной строки:

```sh
kubectl scale worker my-worker --replicas=5       # пишет .spec.replicas через /scale
kubectl autoscale worker my-worker --min=2 --max=10 --cpu-percent=70
kubectl get workers                                # колонки Desired и Ready
```

В контроллере статус пишется через `/status`:

```go
// controller-runtime
err := c.Status().Update(ctx, worker)

// client-go, dynamic client
_, err := dyn.Resource(gvr).Namespace(ns).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
```

`gvr` берётся из того же определения: `workers.Resource.GroupVersionResource("v1")`.

В RBAC `/status` и `/scale` — отдельные ресурсы:

```yaml
rules:
- apiGroups: [example.com]
  resources: [workers]
  verbs: [get, list, watch]
- apiGroups: [example.com]
  resources: [workers/status]
  verbs: [get, update, patch]
- apiGroups: [example.com]         # для HPA и kubectl scale
  resources: [workers/scale]
  verbs: [get, update, patch]
```

Если подресурс не включён, эти вызовы вернут 404, а правила для `workers/status`
и `workers/scale` ничего не дадут.

## Зачем подресурс status

Без подресурса весь объект — и `spec`, и `status` — пишется через один адрес:

- `kubectl replace` или GitOps по манифесту без `status` обнуляет статус,
  выставленный контроллером;
- контроллер, отправляя объект целиком, может вместе со статусом отправить и
  свои изменения `spec`;
- `metadata.generation` растёт при записи статуса, и шаблон
  `status.observedGeneration` зацикливается;
- права на `spec` и `status` не разделить.

С подресурсом основной адрес не меняет `.status`, а `/status` меняет только его.

## Смотрите также

- [crd.md](crd.md) — все настройки CRD и версий.

Этот пример проверяется тестом `Example_statusAndScale`.
