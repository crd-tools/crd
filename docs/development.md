# Устройство и тесты

Для тех, кто меняет сам пакет.

## Пакеты

```
crd.go                доступ к схемам: SetDERSource, Register, Schema, SchemaFor, Of
builder.go            билдер: дерево по экземпляру, поиск поля по адресу, вид правила
rules.go              ручная часть правил: методы Builder, конструкторы, пресеты
rules_kinds.go        интерфейсы и реализации правил: StringOf[S], String, StringField, ...
branches.go           ветки комбинаторов
known.go              известные типы и методы kube-openapi
resource.go           сборка CRD: New, версии, подресурсы, колонки, конверсия, Marshal
defined.go            реестр определений: Add, List
openapi.go            документ OpenAPI v3 с компонентами по типам
modifier.go           модификаторы схем определения: Modifier, ModifierFunc
errors.go             коды ошибок
formats/              форматы OpenAPI
modifiers/            готовые модификаторы: gatewayapi (каналы Gateway API)
validation/           проверка CRD кодом API-сервера
registry/             реестр схем и формат DER
```

## Правила: значение и поле

У каждого вида правила два интерфейса с одинаковыми ограничениями: значение
(`String`) и поле (`StringField`). Ограничения описаны один раз в
дженерик-интерфейсе `StringOf[S]`, где `S` — интерфейс, который методы
возвращают для продолжения цепочки. `String` встраивает `StringOf[String]`, а
`StringField` — `StringOf[StringField]` и `FieldOf[StringField]`. Реализация
тоже общая, поэтому новое ограничение добавляется в одном месте.

## Тесты

```sh
go test -race ./...
```

- `priority_test.go` — все правила старшинства и таблица известных типов, см.
  [schema-sources.md](schema-sources.md);
- `example*_test.go` — исполняемые примеры: видны в godoc и проверяются
  `go test`, каждый печатает получившуюся схему.

Примеры в `docs/` должны иметь двойника среди `Example_*`, чтобы изменение API
ломало тест, а не только документ. Пример из
[status-and-scale.md](status-and-scale.md) — `Example_statusAndScale`.
