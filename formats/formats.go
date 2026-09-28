// Package formats содержит форматы OpenAPI v3, которые API-сервер Kubernetes проверяет в схемах CRD
//
// Список форматов открыт: неизвестный формат API-сервер принимает и игнорирует, он работает как
// документация. Поэтому любая строка тоже годится, константы здесь для автодополнения и защиты
// от опечаток
package formats

// Format это формат OpenAPI v3
type Format string

// Строковые форматы, которые проверяет API-сервер
const (
	BSONObjectID Format = "bsonobjectid" // 24-символьный шестнадцатеричный идентификатор BSON
	URI          Format = "uri"          // URI (net/url.ParseRequestURI)
	Email        Format = "email"        // адрес электронной почты (net/mail.ParseAddress)
	Hostname     Format = "hostname"     // имя хоста в интернете (RFC 1034 §3.1)
	IPv4         Format = "ipv4"         // адрес IPv4 (net.ParseIP)
	IPv6         Format = "ipv6"         // адрес IPv6 (net.ParseIP)
	CIDR         Format = "cidr"         // CIDR (net.ParseCIDR)
	MAC          Format = "mac"          // MAC-адрес (net.ParseMAC)
	UUID         Format = "uuid"         // UUID, заглавные буквы допустимы
	UUID3        Format = "uuid3"        // UUID версии 3
	UUID4        Format = "uuid4"        // UUID версии 4
	UUID5        Format = "uuid5"        // UUID версии 5
	ISBN         Format = "isbn"         // номер ISBN10 или ISBN13
	ISBN10       Format = "isbn10"       // номер ISBN10
	ISBN13       Format = "isbn13"       // номер ISBN13
	CreditCard   Format = "creditcard"   // номер банковской карты
	SSN          Format = "ssn"          // номер социального страхования США
	HexColor     Format = "hexcolor"     // цвет вида "#FFFFFF"
	RGBColor     Format = "rgbcolor"     // цвет вида "rgb(255,255,255)"
	Byte         Format = "byte"         // двоичные данные в base64
	Password     Format = "password"     // любая строка, подсказка скрывать значение
	Date         Format = "date"         // дата вида "2006-01-02" (RFC 3339 full-date)
	Duration     Format = "duration"     // длительность вида "22ns" (time.ParseDuration)
	DateTime     Format = "date-time"    // дата и время (RFC 3339), включает Timestamp в CEL
)

// Числовые форматы
const (
	Int32  Format = "int32"  // целое 32 бита
	Int64  Format = "int64"  // целое 64 бита
	Float  Format = "float"  // число с плавающей точкой 32 бита
	Double Format = "double" // число с плавающей точкой 64 бита
)

// String возвращает формат строкой
func (f Format) String() string {
	return string(f)
}
