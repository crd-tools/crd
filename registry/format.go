package registry

import (
	"crypto/sha256"
	"encoding/asn1"
	"errors"
	"fmt"
	"strings"
)

// FormatVersion версия формата файлов реестра
const FormatVersion = 2

// ErrFormat файл другой версии формата
var ErrFormat = errors.New("unsupported format version")

// IndexFile имя файла-манифеста в каталоге данных
const IndexFile = "index.der"

// FileExt расширение файлов со схемами пакетов
const FileExt = ".der"

// File это схемы типов одного Go-пакета
type File struct {
	// Format версия формата файла
	Format int

	// Package путь импорта пакета, которому принадлежат типы
	Package string

	// Module модуль, в котором лежит пакет
	Module string

	// Version версия модуля или хэш исходников для локальных пакетов
	Version string

	// Generator версия генератора, построившего схемы
	Generator string

	// Types схемы типов, отсортированы по имени
	Types []Type
}

// Type это схема одного типа
type Type struct {
	// Name имя типа внутри пакета
	Name string

	// Refs ключи типов, на которые ссылается схема, отсортированы
	Refs []string

	// Schema apiextensions/v1 JSONSchemaProps в protobuf-кодировке Kubernetes
	Schema []byte
}

// Index это манифест реестра
type Index struct {
	// Format версия формата файла
	Format int

	// Generator версия генератора, построившего реестр
	Generator string

	// Roots типы, явно запрошенные метками в коде, отсортированы по ключу
	Roots []Root

	// Files имена файлов со схемами пакетов, отсортированы
	Files []string

	// Types расположение всех типов по файлам, отсортированы по ключу
	Types []Entry
}

// Entry это расположение схемы типа
type Entry struct {
	// Type ключ типа
	Type string

	// File имя файла, в котором лежит схема
	File string

	// Hash хэш схемы, по нему проверяются конфликты без чтения файла
	Hash []byte
}

// Root это тип, запрошенный меткой в коде
type Root struct {
	// Type ключ типа
	Type string

	// Usages места в коде, где стоит метка, отсортированы
	Usages []string
}

// MarshalFile кодирует файл пакета в DER
func MarshalFile(f File) ([]byte, error) {
	f.Format = FormatVersion
	return asn1.Marshal(f)
}

// UnmarshalFile декодирует файл пакета из DER
//
//	ErrFormat
func UnmarshalFile(data []byte) (File, error) {
	if err := checkFormat(data); err != nil {
		return File{}, err
	}
	var f File
	if err := unmarshal(data, &f); err != nil {
		return File{}, err
	}
	return f, nil
}

// MarshalIndex кодирует манифест в DER
func MarshalIndex(idx Index) ([]byte, error) {
	idx.Format = FormatVersion
	return asn1.Marshal(idx)
}

// UnmarshalIndex декодирует манифест из DER
//
//	ErrFormat
func UnmarshalIndex(data []byte) (Index, error) {
	if err := checkFormat(data); err != nil {
		return Index{}, err
	}
	var idx Index
	if err := unmarshal(data, &idx); err != nil {
		return Index{}, err
	}
	return idx, nil
}

// checkFormat читает только номер формата из первого элемента последовательности
// Так файл другой версии распознаётся, даже если его поля не совпадают с текущими
func checkFormat(data []byte) error {
	var seq asn1.RawValue
	if _, err := asn1.Unmarshal(data, &seq); err != nil {
		return err
	}
	if seq.Class != asn1.ClassUniversal || seq.Tag != asn1.TagSequence {
		return errors.New("not a DER sequence")
	}
	var format int
	if _, err := asn1.Unmarshal(seq.Bytes, &format); err != nil {
		return err
	}
	if format != FormatVersion {
		return fmt.Errorf("%w %d", ErrFormat, format)
	}
	return nil
}

// Hash возвращает хэш схемы для индекса
func Hash(schema []byte) []byte {
	sum := sha256.Sum256(schema)
	return sum[:16]
}

func unmarshal(data []byte, v any) error {
	rest, err := asn1.Unmarshal(data, v)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return fmt.Errorf("trailing data: %d bytes", len(rest))
	}
	return nil
}

// FileName возвращает имя файла со схемами пакета
func FileName(pkg string) string {
	var b strings.Builder
	for _, r := range pkg {
		switch {
		case r == '/':
			b.WriteByte('~')
		case r == '~':
			b.WriteString("~~")
		default:
			b.WriteRune(r)
		}
	}
	return b.String() + FileExt
}

// Key возвращает ключ типа в реестре
func Key(pkg, name string) string {
	return pkg + "." + name
}

// SplitKey разделяет ключ типа на путь пакета и имя типа
func SplitKey(key string) (pkg, name string, err error) {
	i := strings.LastIndexByte(key, '.')
	slash := strings.LastIndexByte(key, '/')
	if i <= 0 || i < slash || i == len(key)-1 {
		return "", "", errors.New("invalid type key: " + key)
	}
	return key[:i], key[i+1:], nil
}
