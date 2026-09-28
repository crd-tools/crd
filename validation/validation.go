// Package validation проверяет CustomResourceDefinition кодом API-сервера Kubernetes
//
// Проверка вынесена из пакета crd, потому что тянет вдвое больше зависимостей,
// а приложениям, которым нужны только схемы, она не нужна
package validation

import (
	"context"

	"github.com/mantyr/codes"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crdvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"

	"crd.tools/crd"
)

// Validate проверяет CRD так же, как API-сервер при создании: структурность схем,
// подресурсы, колонки kubectl, конверсию, имена и версии
// Перед проверкой применяются значения по умолчанию v1, как это делает API-сервер,
// например порт 443 у сервиса вебхука. Сам c не изменяется
// Хранимые версии для проверки берутся из версии хранения, как у нового CRD
//
//	crd.InvalidArgument
//	crd.SchemaConvert
//	crd.InvalidCRD
func Validate(ctx context.Context, c *apiextv1.CustomResourceDefinition) error {
	if c == nil {
		return codes.NewError(crd.InvalidArgument, "validation: nil CRD")
	}
	c = c.DeepCopy()
	apiextv1.SetObjectDefaults_CustomResourceDefinition(c)
	internal := &apiext.CustomResourceDefinition{}
	if err := apiextv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(c, internal, nil); err != nil {
		return codes.NewError(crd.SchemaConvert, "validation: convert CRD: %s", err)
	}
	if len(internal.Status.StoredVersions) == 0 {
		for _, v := range internal.Spec.Versions {
			if v.Storage {
				internal.Status.StoredVersions = append(internal.Status.StoredVersions, v.Name)
			}
		}
	}
	if errs := crdvalidation.ValidateCustomResourceDefinition(ctx, internal); len(errs) > 0 {
		return codes.NewError(crd.InvalidCRD, "validation: %s", errs.ToAggregate())
	}
	return nil
}
