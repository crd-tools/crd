package crd_test

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"crd.tools/crd"
	"crd.tools/crd/validation"
)

// printSchema печатает схему типа T в YAML, ключи отсортированы
func printSchema[T any]() {
	s, err := crd.SchemaFor[T]()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	out, err := yaml.Marshal(s)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Print(string(out))
}

// basicSpec это плоская спецификация пользователя
type basicSpec struct {
	ClientID string `json:"clientId"`
	Name     string `json:"name"`
	TTL      int    `json:"ttl,omitempty"`
}

func (s *basicSpec) CRD(b crd.Builder) error {
	b.K8sName(&s.ClientID).MaxLength(64).Description("уникальный идентификатор клиента")
	b.String(&s.Name).MaxLength(128).Description("имя для людей")
	b.DurationSeconds(&s.TTL, 60, 3600).Default(900).Description("время жизни в секундах")
	return nil
}

// Example_basic собирает полный CRD из плоской структуры
// Правила задаются для полей по указателям, пресеты дают типовые ограничения
func Example_basic() {
	c := crd.New[basicSpec]().
		Group("example.com").Kind("User").Plural("users").
		Version("v1alpha1", true, true)

	if err := c.Encode(os.Stdout); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// ---
	// apiVersion: apiextensions.k8s.io/v1
	// kind: CustomResourceDefinition
	// metadata:
	//   name: users.example.com
	// spec:
	//   group: example.com
	//   names:
	//     kind: User
	//     listKind: UserList
	//     plural: users
	//     singular: user
	//   scope: Namespaced
	//   versions:
	//   - name: v1alpha1
	//     schema:
	//       openAPIV3Schema:
	//         properties:
	//           apiVersion:
	//             type: string
	//           clientId:
	//             description: уникальный идентификатор клиента
	//             maxLength: 64
	//             pattern: ^[a-z0-9-]+$
	//             type: string
	//           kind:
	//             type: string
	//           metadata:
	//             type: object
	//           name:
	//             description: имя для людей
	//             maxLength: 128
	//             type: string
	//           ttl:
	//             default: 900
	//             description: время жизни в секундах
	//             maximum: 3600
	//             minimum: 60
	//             type: integer
	//         required:
	//         - clientId
	//         - name
	//         type: object
	//     served: true
	//     storage: true
}

// resourceSpec это спецификация ресурса
type resourceSpec struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}

func (s *resourceSpec) CRD(b crd.Builder) error {
	b.String(&s.Name).MinLength(1).Description("имя пользователя")
	b.Port(&s.Port).Description("TCP-порт")
	return nil
}

// resourceUser это полный тип ресурса с TypeMeta и ObjectMeta
type resourceUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec resourceSpec `json:"spec"`
}

// Example_resource собирает CRD из полного типа ресурса
// TypeMeta даёт apiVersion и kind, ObjectMeta — metadata, поле Spec — ключ spec
func Example_resource() {
	obj, err := crd.New[resourceUser]().
		Group("example.com").Kind("User").Plural("users").
		Version("v1", true, true).
		Build()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	if err := validation.Validate(context.Background(), obj); err != nil {
		fmt.Println("error:", err)
		return
	}
	out, _ := yaml.Marshal(obj.Spec.Versions[0].Schema.OpenAPIV3Schema)
	fmt.Print(string(out))
	// Output:
	// properties:
	//   apiVersion:
	//     type: string
	//   kind:
	//     type: string
	//   metadata:
	//     type: object
	//   spec:
	//     properties:
	//       name:
	//         description: имя пользователя
	//         minLength: 1
	//         type: string
	//       port:
	//         description: TCP-порт
	//         format: int32
	//         maximum: 65535
	//         minimum: 1
	//         type: integer
	//     required:
	//     - name
	//     - port
	//     type: object
	// required:
	// - spec
	// type: object
}

// nestedJWK это открытый ключ, описывает свою схему сам
type nestedJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
}

func (s *nestedJWK) CRD(b crd.Builder) error {
	b.String(&s.Kty).Enum("RSA").Description("тип ключа")
	b.String(&s.Kid).MaxLength(64).Description("идентификатор ключа")
	return nil
}

// nestedSpec собирает вложенные виды полей
type nestedSpec struct {
	Keys   []nestedJWK       `json:"keys"`
	Labels map[string]string `json:"labels"`
	Extra  any               `json:"extra"`
}

func (s *nestedSpec) CRD(b crd.Builder) error {
	b.Array(&s.Keys).MinItems(1).MaxItems(5).Description("открытые ключи")
	b.Map(&s.Labels).MaxProperties(10).Values(crd.NewString().MaxLength(63)).Description("метки")
	return nil
}

// Example_nested показывает массив структур, мапу и произвольный JSON
// Элемент массива описан на своём типе и раскрыт из $ref, значение мапы задано правилом
func Example_nested() {
	printSchema[nestedSpec]()
	// Output:
	// properties:
	//   extra:
	//     x-kubernetes-preserve-unknown-fields: true
	//   keys:
	//     description: открытые ключи
	//     items:
	//       properties:
	//         kid:
	//           description: идентификатор ключа
	//           maxLength: 64
	//           type: string
	//         kty:
	//           description: тип ключа
	//           enum:
	//           - RSA
	//           type: string
	//       required:
	//       - kid
	//       - kty
	//       type: object
	//     maxItems: 5
	//     minItems: 1
	//     type: array
	//   labels:
	//     additionalProperties:
	//       maxLength: 63
	//       type: string
	//     description: метки
	//     maxProperties: 10
	//     type: object
	// required:
	// - extra
	// - keys
	// - labels
	// type: object
}
