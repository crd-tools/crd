package crd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mantyr/codes"

	"crd.tools/crd/registry"
)

// Все ошибки пакета несут код, его проверяют через Code(err), а не по тексту
// codes.CodeOf не разворачивает обёртки, поэтому при обёртке код переносится явно

// Базовые коды из пакета codes
const (
	OK              = codes.OK
	Unknown         = codes.Unknown
	InvalidArgument = codes.InvalidArgument
	NotFound        = codes.NotFound
	Unimplemented   = codes.Unimplemented
	Internal        = codes.Internal
)

// Коды пакета, начинаются после codes.CustomCodes
const (
	// NotStruct тип должен быть структурой
	NotStruct codes.Code = iota + codes.CustomCodes + 1

	// FieldNotFound указатель не ведёт на поле описываемой структуры
	FieldNotFound

	// RuleKindMismatch вид правила не подходит к Go-типу поля
	RuleKindMismatch

	// UnsupportedType Go-тип нельзя отобразить в схему
	UnsupportedType

	// RecursiveType тип ссылается сам на себя
	RecursiveType

	// NotDescribed тип не реализует ни Describer, ни Ruler
	NotDescribed

	// InvalidRule правило задано неверно
	InvalidRule

	// SourceLoad не удалось прочитать каталог снимков
	SourceLoad

	// SchemaConflict схемы одного типа из разных источников не совпадают
	SchemaConflict

	// SchemaConvert не удалось преобразовать CRD во внутреннее представление
	SchemaConvert

	// InvalidCRD CRD не проходит проверку API-сервера
	InvalidCRD

	// ModifierFailed модификатор схемы вернул ошибку
	ModifierFailed
)

// Code возвращает код ошибки, OK для nil и Unknown для чужих ошибок
func Code(err error) codes.Code {
	return codes.CodeOf(err)
}

// newError создаёт ошибку с кодом, аргументы как в codes.NewError
func newError(code codes.Code, args ...any) error {
	return codes.NewError(code, args...)
}

// wrap добавляет контекст к ошибке, сохраняя её код
// Ошибки реестра переводятся в коды пакета
func wrap(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if format != "" {
		msg = fmt.Sprintf(format, args...) + ": " + msg
	}
	return newError(codeOf(err), msg)
}

// codeOf возвращает код ошибки, распознавая метки реестра
func codeOf(err error) codes.Code {
	if code := Code(err); code != Unknown {
		return code
	}
	switch {
	case errors.Is(err, registry.ErrNotFound):
		return NotFound
	case errors.Is(err, registry.ErrConflict):
		return SchemaConflict
	case errors.Is(err, registry.ErrRecursive):
		return RecursiveType
	case errors.Is(err, registry.ErrFormat):
		return SourceLoad
	case errors.Is(err, registry.ErrModify):
		return ModifierFailed
	}
	return Unknown
}

// joinErrors собирает несколько ошибок в одну с кодом первой
func joinErrors(title string, errs []error) error {
	lines := make([]string, 0, len(errs))
	for _, err := range errs {
		lines = append(lines, err.Error())
	}
	return newError(Code(errs[0]), title+":\n  "+strings.Join(lines, "\n  "))
}
