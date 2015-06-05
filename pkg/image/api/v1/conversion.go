package v1

import (
	"fmt"
	"sort"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/image/api"
)

func init() {
	err := kapi.Scheme.AddConversionFuncs(
		// The docker metadata must be cast to a version
		func(in *newer.Image, out *Image, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}

			out.DockerImageReference = in.DockerImageReference
			out.DockerImageManifest = in.DockerImageManifest

			version := in.DockerImageMetadataVersion
			if len(version) == 0 {
				version = "1.0"
			}
			data, err := kapi.Scheme.EncodeToVersion(&in.DockerImageMetadata, version)
			if err != nil {
				return err
			}
			out.DockerImageMetadata.RawJSON = data
			out.DockerImageMetadataVersion = version

			return nil
		},
		func(in *Image, out *newer.Image, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}

			out.DockerImageReference = in.DockerImageReference
			out.DockerImageManifest = in.DockerImageManifest

			version := in.DockerImageMetadataVersion
			if len(version) == 0 {
				version = "1.0"
			}
			if len(in.DockerImageMetadata.RawJSON) > 0 {
				// TODO: add a way to default the expected kind and version of an object if not set
				obj, err := kapi.Scheme.New(version, "DockerImage")
				if err != nil {
					return err
				}
				if err := kapi.Scheme.DecodeInto(in.DockerImageMetadata.RawJSON, obj); err != nil {
					return err
				}
				if err := s.Convert(obj, &out.DockerImageMetadata, 0); err != nil {
					return err
				}
			}
			out.DockerImageMetadataVersion = version

			return nil
		},

		func(in *[]NamedTagEventList, out *map[string]newer.TagEventList, s conversion.Scope) error {
			for _, curr := range *in {
				newTagEventList := newer.TagEventList{}
				if err := s.Convert(&curr.Items, &newTagEventList.Items, 0); err != nil {
					return err
				}
				(*out)[curr.Tag] = newTagEventList
			}

			return nil
		},
		func(in *map[string]newer.TagEventList, out *[]NamedTagEventList, s conversion.Scope) error {
			allKeys := make([]string, 0, len(*in))
			for key := range *in {
				allKeys = append(allKeys, key)
			}
			sort.Strings(allKeys)

			for _, key := range allKeys {
				newTagEventList := (*in)[key]
				oldTagEventList := &NamedTagEventList{Tag: key}
				if err := s.Convert(&newTagEventList.Items, &oldTagEventList.Items, 0); err != nil {
					return err
				}

				*out = append(*out, *oldTagEventList)
			}

			return nil
		},
		func(in *[]NamedTagReference, out *map[string]newer.TagReference, s conversion.Scope) error {
			for _, curr := range *in {
				r := newer.TagReference{
					Annotations: curr.Annotations,
				}
				if err := s.Convert(&curr.From, &r.From, 0); err != nil {
					return err
				}
				(*out)[curr.Name] = r
			}
			return nil
		},
		func(in *map[string]newer.TagReference, out *[]NamedTagReference, s conversion.Scope) error {
			allTags := make([]string, 0, len(*in))
			for tag := range *in {
				allTags = append(allTags, tag)
			}
			sort.Strings(allTags)

			for _, tag := range allTags {
				newTagReference := (*in)[tag]
				oldTagReference := NamedTagReference{
					Name:        tag,
					Annotations: newTagReference.Annotations,
				}
				if err := s.Convert(&newTagReference.From, &oldTagReference.From, 0); err != nil {
					return err
				}
				*out = append(*out, oldTagReference)
			}
			return nil
		},

		func(in *ImageStreamSpec, out *newer.ImageStreamSpec, s conversion.Scope) error {
			out.DockerImageRepository = in.DockerImageRepository
			out.Tags = make(map[string]newer.TagReference)
			return s.Convert(&in.Tags, &out.Tags, 0)
		},
		func(in *newer.ImageStreamSpec, out *ImageStreamSpec, s conversion.Scope) error {
			out.DockerImageRepository = in.DockerImageRepository
			out.Tags = make([]NamedTagReference, 0, 0)
			return s.Convert(&in.Tags, &out.Tags, 0)
		},
		func(in *ImageStreamStatus, out *newer.ImageStreamStatus, s conversion.Scope) error {
			out.DockerImageRepository = in.DockerImageRepository
			out.Tags = make(map[string]newer.TagEventList)
			return s.Convert(&in.Tags, &out.Tags, 0)
		},
		func(in *newer.ImageStreamStatus, out *ImageStreamStatus, s conversion.Scope) error {
			out.DockerImageRepository = in.DockerImageRepository
			out.Tags = make([]NamedTagEventList, 0, 0)
			return s.Convert(&in.Tags, &out.Tags, 0)
		},
		func(in *newer.ImageStreamMapping, out *ImageStreamMapping, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.DestFromSource)
		},
		func(in *ImageStreamMapping, out *newer.ImageStreamMapping, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.SourceToDest)
		},

		// convert internal deprecated ImageRepository objects to Stream objects
		func(in *newer.ImageRepository, out *ImageStream, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}
			out.Spec.DockerImageRepository = in.DockerImageRepository
			for tag, value := range in.Tags {
				out.Spec.Tags = append(out.Spec.Tags, NamedTagReference{
					Name: tag,
					From: &v1.ObjectReference{
						Name: value,
					},
				})
			}
			return s.Convert(&in.Status, &out.Status, 0)
		},
		func(in *ImageStream, out *newer.ImageRepository, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}

			out.DockerImageRepository = in.Spec.DockerImageRepository
			if in.Spec.Tags != nil && out.Tags == nil {
				out.Tags = make(map[string]string)
			}
			for _, namedTagReference := range in.Spec.Tags {
				if namedTagReference.From != nil {
					out.Tags[namedTagReference.Name] = namedTagReference.From.Name
				}
			}

			return s.Convert(&in.Status, &out.Status, 0)
		},
		func(in *ImageStreamMapping, out *newer.ImageRepositoryMapping, s conversion.Scope) error {
			if err := s.Convert(&in.ObjectMeta, &out.ObjectMeta, 0); err != nil {
				return err
			}
			out.Tag = in.Tag
			return s.Convert(&in.Image, &out.Image, 0)
		},
		func(in *newer.ImageRepositoryStatus, out *ImageStreamStatus, s conversion.Scope) error {
			out.DockerImageRepository = in.DockerImageRepository
			return s.Convert(&in.Tags, &out.Tags, 0)
		},
		func(in *ImageStreamStatus, out *newer.ImageRepositoryStatus, s conversion.Scope) error {
			out.DockerImageRepository = in.DockerImageRepository
			out.Tags = make(map[string]newer.TagEventList)
			return s.Convert(&in.Tags, &out.Tags, 0)
		},
		func(in *newer.ImageRepositoryMapping, out *ImageStreamMapping, s conversion.Scope) error {
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields|conversion.AllowDifferentFieldTypeNames)
		},
		func(in *newer.ImageRepositoryList, out *ImageStreamList, s conversion.Scope) error {
			if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
				return err
			}
			return s.Convert(&in.Items, &out.Items, 0)
		},
		func(in *ImageStreamList, out *newer.ImageRepositoryList, s conversion.Scope) error {
			if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
				return err
			}
			return s.Convert(&in.Items, &out.Items, 0)
		},
		func(in *newer.ImageRepositoryTag, out *ImageStreamTag, s conversion.Scope) error {
			return s.Convert(&in.Image, &out.Image, conversion.IgnoreMissingFields)
		},
		func(in *ImageStreamTag, out *newer.ImageRepositoryTag, s conversion.Scope) error {
			return s.Convert(&in.Image, &out.Image, conversion.IgnoreMissingFields)
		},
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}

	err = kapi.Scheme.AddFieldLabelConversionFunc("v1", "ImageStream",
		func(label, value string) (string, string, error) {
			switch label {
			case "name":
				return "metadata.name", value, nil
			case "metadata.name", "spec.dockerImageRepository", "status.dockerImageRepository":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
