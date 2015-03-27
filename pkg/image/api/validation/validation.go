package validation

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/image/api"
)

// ValidateImage tests required fields for an Image.
func ValidateImage(image *api.Image) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	if len(image.Name) == 0 {
		result = append(result, fielderrors.NewFieldRequired("name"))
	}
	if len(image.DockerImageReference) == 0 {
		result = append(result, fielderrors.NewFieldRequired("dockerImageReference"))
	} else {
		if _, err := api.ParseDockerImageReference(image.DockerImageReference); err != nil {
			result = append(result, fielderrors.NewFieldInvalid("dockerImageReference", image.DockerImageReference, err.Error()))
		}
	}

	return result
}

// ValidateImageRepository tests required fields for an ImageRepository.
func ValidateImageRepository(repo *api.ImageRepository) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	if repo.Tags == nil {
		repo.Tags = make(map[string]string)
	}
	if len(repo.Name) == 0 {
		result = append(result, fielderrors.NewFieldRequired("name"))
	}
	if !util.IsDNS1123Subdomain(repo.Namespace) {
		result = append(result, fielderrors.NewFieldInvalid("namespace", repo.Namespace, ""))
	}
	if len(repo.DockerImageRepository) != 0 {
		if _, err := api.ParseDockerImageReference(repo.DockerImageRepository); err != nil {
			result = append(result, fielderrors.NewFieldInvalid("dockerImageRepository", repo.DockerImageRepository, err.Error()))
		}
	}

	return result
}

func ValidateImageRepositoryUpdate(newRepo, oldRepo *api.ImageRepository) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	result = append(result, validation.ValidateObjectMetaUpdate(&oldRepo.ObjectMeta, &newRepo.ObjectMeta).Prefix("metadata")...)
	result = append(result, ValidateImageRepository(newRepo)...)

	return result
}

func ValidateImageRepositoryStatusUpdate(newRepo, oldRepo *api.ImageRepository) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	result = append(result, validation.ValidateObjectMetaUpdate(&oldRepo.ObjectMeta, &newRepo.ObjectMeta).Prefix("metadata")...)
	newRepo.Tags = oldRepo.Tags
	newRepo.DockerImageRepository = oldRepo.DockerImageRepository
	return result
}

// ValidateImageRepositoryMapping tests required fields for an ImageRepositoryMapping.
func ValidateImageRepositoryMapping(mapping *api.ImageRepositoryMapping) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	hasRepository := len(mapping.DockerImageRepository) != 0
	hasName := len(mapping.Name) != 0
	switch {
	case hasRepository:
		if _, err := api.ParseDockerImageReference(mapping.DockerImageRepository); err != nil {
			result = append(result, fielderrors.NewFieldInvalid("dockerImageRepository", mapping.DockerImageRepository, err.Error()))
		}
	case hasName:
	default:
		result = append(result, fielderrors.NewFieldRequired("name"))
		result = append(result, fielderrors.NewFieldRequired("dockerImageRepository"))
	}

	if !util.IsDNS1123Subdomain(mapping.Namespace) {
		result = append(result, fielderrors.NewFieldInvalid("namespace", mapping.Namespace, ""))
	}
	if len(mapping.Tag) == 0 {
		result = append(result, fielderrors.NewFieldRequired("tag"))
	}
	if errs := ValidateImage(&mapping.Image).Prefix("image"); len(errs) != 0 {
		result = append(result, errs...)
	}
	return result
}
