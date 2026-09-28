# crd

[![Go Reference](https://pkg.go.dev/badge/crd.tools/crd.svg)](https://pkg.go.dev/crd.tools/crd)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE.md)

Схемы Go-типов и CustomResourceDefinition для Kubernetes — кодом, без маркеров в
комментариях.

- Ограничения полей описываются методом `CRD()` на типе, поля передаются
  указателями, опечатки ловит компилятор.
- Из типа собирается CRD: версии, подресурсы, колонки kubectl, конверсия.
- Схемы доступны и в рантайме приложения, а не только в YAML.
- Документ OpenAPI для SDK, где каждый Go-тип — отдельный компонент.
- Схемы чужих типов из kubebuilder-маркеров через встроенные снимки.

## Быстрый старт

Тип с описанием полей:

```go
package apps

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"crd.tools/crd"
)

type AppSpec struct {
	Replicas int32  `json:"replicas"`
	Image    string `json:"image"`
}

func (s *AppSpec) CRD(b crd.Builder) error {
	b.Integer(&s.Replicas).Minimum(0).Maximum(10)
	b.NonEmptyString(&s.Image).Description("образ контейнера")
	return nil
}

type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AppSpec `json:"spec"`
}
```

Определение CRD и его регистрация:

```go
var Resource = crd.New[App]().
	Group("example.com").Kind("App").Plural("apps").
	Version("v1", true, true)

func init() {
	crd.Add(Resource)
}
```

Манифест:

```go
err := apps.Resource.Encode(os.Stdout)
```

```yaml
spec:
  group: example.com
  names:
    kind: App
    plural: apps
  versions:
  - name: v1
    schema:
      openAPIV3Schema:
        properties:
          spec:
            properties:
              image:
                description: образ контейнера
                minLength: 1
                type: string
              replicas:
                format: int32
                maximum: 10
                minimum: 0
                type: integer
            required:
            - image
            - replicas
            type: object
```

Та же схема доступна в коде: `crd.SchemaFor[apps.App]()`, а группа, версия и
ресурс — у определения: `apps.Resource.GroupVersionResource("v1")`.

## Установка

Нужен Go 1.26 или новее.

```sh
go get crd.tools/crd
```

## Документация

| Хочу | Читать |
|---|---|
| описать поля: ограничения, обязательность, пресеты, CEL | [docs/rules.md](docs/rules.md) |
| понять, откуда берётся схема поля и что важнее | [docs/schema-sources.md](docs/schema-sources.md) |
| собрать CRD: версии, колонки, конверсия, проверка | [docs/crd.md](docs/crd.md) |
| ресурс со status и scale — пример по шагам | [docs/status-and-scale.md](docs/status-and-scale.md) |
| подключить схему чужого типа из kubebuilder-маркеров | [docs/kubebuild.md](docs/kubebuild.md) |
| получить OpenAPI с отдельными типами для SDK | [docs/openapi.md](docs/openapi.md) |
| разобрать ошибку по коду | [docs/errors.md](docs/errors.md) |
| поменять сам пакет | [docs/development.md](docs/development.md) |

Исполняемые примеры — функции `Example_*` в godoc: каждый печатает получившуюся
схему.

## Пакеты

| Пакет | Для чего |
|---|---|
| `crd.tools/crd` | описание схем, сборка CRD, схемы в рантайме |
| `crd.tools/crd/formats` | форматы OpenAPI: `formats.DateTime`, `formats.Email`, ... |
| `crd.tools/crd/modifiers/gatewayapi` | каналы Gateway API для чужих CRD: `Standard()`, `Experimental()` |
| `crd.tools/crd/validation` | проверка CRD кодом API-сервера |
| `crd.tools/crd/registry` | реестр схем и формат снимков DER |

## Author

[Oleg Shevelev][mantyr]

[mantyr]: https://github.com/mantyr

