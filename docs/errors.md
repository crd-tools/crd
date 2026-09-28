# Ошибки

Все ошибки пакета несут коды [github.com/mantyr/codes](https://github.com/mantyr/codes).
Проверяйте код, а не текст:

```go
if crd.Code(err) == crd.RuleKindMismatch {
	...
}
```

| Код | Когда |
|---|---|
| `InvalidArgument` | неверные аргументы: пустая группа, `nil`, не именованный тип, разные схемы общего компонента в OpenAPI |
| `NotFound` | схемы типа нет ни в снимках, ни в коде |
| `NotStruct` | тип ресурса или `Describer` не структура |
| `FieldNotFound` | указатель не ведёт на поле описываемой структуры |
| `RuleKindMismatch` | вид правила не подходит к JSON-виду поля |
| `UnsupportedType` | тип не отображается в схему, в том числе свой `MarshalJSON` без описания |
| `RecursiveType` | тип ссылается сам на себя |
| `NotDescribed` | `Register` для типа без `Describer` и `Ruler` |
| `InvalidRule` | правило задано неверно, например `nil` в `Items` |
| `SourceLoad` | не читается каталог снимков |
| `SchemaConflict` | схемы одного типа из разных источников не совпадают |
| `SchemaConvert`, `InvalidCRD` | ошибки `validation.Validate` |
| `ModifierFailed` | модификатор схемы вернул ошибку |

Если `CRD()` накопил несколько ошибок, они возвращаются одной ошибкой с кодом
первой, каждая строка — с местом вызова:

```
crd: badKinds.CRD:
  user.go:14: badKinds.N: string rule is not applicable to int
  user.go:15: *string is not a pointer to a field of badKinds
```
