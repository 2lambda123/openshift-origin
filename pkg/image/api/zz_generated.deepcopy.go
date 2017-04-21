// +build !ignore_autogenerated_openshift

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package api

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	pkg_api "k8s.io/kubernetes/pkg/api"
	reflect "reflect"
)

func init() {
	SchemeBuilder.Register(RegisterDeepCopies)
}

// RegisterDeepCopies adds deep-copy functions to the given scheme. Public
// to allow building arbitrary schemes.
func RegisterDeepCopies(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedDeepCopyFuncs(
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_Descriptor, InType: reflect.TypeOf(&Descriptor{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerConfig, InType: reflect.TypeOf(&DockerConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerConfigHistory, InType: reflect.TypeOf(&DockerConfigHistory{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerConfigRootFS, InType: reflect.TypeOf(&DockerConfigRootFS{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerFSLayer, InType: reflect.TypeOf(&DockerFSLayer{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerHistory, InType: reflect.TypeOf(&DockerHistory{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerImage, InType: reflect.TypeOf(&DockerImage{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerImageConfig, InType: reflect.TypeOf(&DockerImageConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerImageManifest, InType: reflect.TypeOf(&DockerImageManifest{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerImageReference, InType: reflect.TypeOf(&DockerImageReference{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerV1CompatibilityImage, InType: reflect.TypeOf(&DockerV1CompatibilityImage{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_DockerV1CompatibilityImageSize, InType: reflect.TypeOf(&DockerV1CompatibilityImageSize{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_Image, InType: reflect.TypeOf(&Image{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageImportSpec, InType: reflect.TypeOf(&ImageImportSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageImportStatus, InType: reflect.TypeOf(&ImageImportStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageLayer, InType: reflect.TypeOf(&ImageLayer{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageList, InType: reflect.TypeOf(&ImageList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageSignature, InType: reflect.TypeOf(&ImageSignature{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStream, InType: reflect.TypeOf(&ImageStream{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamImage, InType: reflect.TypeOf(&ImageStreamImage{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamImport, InType: reflect.TypeOf(&ImageStreamImport{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamImportSpec, InType: reflect.TypeOf(&ImageStreamImportSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamImportStatus, InType: reflect.TypeOf(&ImageStreamImportStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamList, InType: reflect.TypeOf(&ImageStreamList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamMapping, InType: reflect.TypeOf(&ImageStreamMapping{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamSpec, InType: reflect.TypeOf(&ImageStreamSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamStatus, InType: reflect.TypeOf(&ImageStreamStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamTag, InType: reflect.TypeOf(&ImageStreamTag{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_ImageStreamTagList, InType: reflect.TypeOf(&ImageStreamTagList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_RepositoryImportSpec, InType: reflect.TypeOf(&RepositoryImportSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_RepositoryImportStatus, InType: reflect.TypeOf(&RepositoryImportStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_SignatureCondition, InType: reflect.TypeOf(&SignatureCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_SignatureGenericEntity, InType: reflect.TypeOf(&SignatureGenericEntity{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_SignatureIssuer, InType: reflect.TypeOf(&SignatureIssuer{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_SignatureSubject, InType: reflect.TypeOf(&SignatureSubject{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_TagEvent, InType: reflect.TypeOf(&TagEvent{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_TagEventCondition, InType: reflect.TypeOf(&TagEventCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_TagEventList, InType: reflect.TypeOf(&TagEventList{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_TagImportPolicy, InType: reflect.TypeOf(&TagImportPolicy{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_TagReference, InType: reflect.TypeOf(&TagReference{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_api_TagReferencePolicy, InType: reflect.TypeOf(&TagReferencePolicy{})},
	)
}

func DeepCopy_api_Descriptor(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*Descriptor)
		out := out.(*Descriptor)
		*out = *in
		return nil
	}
}

func DeepCopy_api_DockerConfig(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerConfig)
		out := out.(*DockerConfig)
		*out = *in
		if in.PortSpecs != nil {
			in, out := &in.PortSpecs, &out.PortSpecs
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.ExposedPorts != nil {
			in, out := &in.ExposedPorts, &out.ExposedPorts
			*out = make(map[string]struct{})
			for key := range *in {
				(*out)[key] = struct{}{}
			}
		}
		if in.Env != nil {
			in, out := &in.Env, &out.Env
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.Cmd != nil {
			in, out := &in.Cmd, &out.Cmd
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.DNS != nil {
			in, out := &in.DNS, &out.DNS
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.Volumes != nil {
			in, out := &in.Volumes, &out.Volumes
			*out = make(map[string]struct{})
			for key := range *in {
				(*out)[key] = struct{}{}
			}
		}
		if in.Entrypoint != nil {
			in, out := &in.Entrypoint, &out.Entrypoint
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.SecurityOpts != nil {
			in, out := &in.SecurityOpts, &out.SecurityOpts
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.OnBuild != nil {
			in, out := &in.OnBuild, &out.OnBuild
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.Labels != nil {
			in, out := &in.Labels, &out.Labels
			*out = make(map[string]string)
			for key, val := range *in {
				(*out)[key] = val
			}
		}
		return nil
	}
}

func DeepCopy_api_DockerConfigHistory(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerConfigHistory)
		out := out.(*DockerConfigHistory)
		*out = *in
		out.Created = in.Created.DeepCopy()
		return nil
	}
}

func DeepCopy_api_DockerConfigRootFS(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerConfigRootFS)
		out := out.(*DockerConfigRootFS)
		*out = *in
		if in.DiffIDs != nil {
			in, out := &in.DiffIDs, &out.DiffIDs
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		return nil
	}
}

func DeepCopy_api_DockerFSLayer(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerFSLayer)
		out := out.(*DockerFSLayer)
		*out = *in
		return nil
	}
}

func DeepCopy_api_DockerHistory(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerHistory)
		out := out.(*DockerHistory)
		*out = *in
		return nil
	}
}

func DeepCopy_api_DockerImage(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerImage)
		out := out.(*DockerImage)
		*out = *in
		out.Created = in.Created.DeepCopy()
		if err := DeepCopy_api_DockerConfig(&in.ContainerConfig, &out.ContainerConfig, c); err != nil {
			return err
		}
		if in.Config != nil {
			in, out := &in.Config, &out.Config
			*out = new(DockerConfig)
			if err := DeepCopy_api_DockerConfig(*in, *out, c); err != nil {
				return err
			}
		}
		return nil
	}
}

func DeepCopy_api_DockerImageConfig(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerImageConfig)
		out := out.(*DockerImageConfig)
		*out = *in
		out.Created = in.Created.DeepCopy()
		if err := DeepCopy_api_DockerConfig(&in.ContainerConfig, &out.ContainerConfig, c); err != nil {
			return err
		}
		if in.Config != nil {
			in, out := &in.Config, &out.Config
			*out = new(DockerConfig)
			if err := DeepCopy_api_DockerConfig(*in, *out, c); err != nil {
				return err
			}
		}
		if in.RootFS != nil {
			in, out := &in.RootFS, &out.RootFS
			*out = new(DockerConfigRootFS)
			if err := DeepCopy_api_DockerConfigRootFS(*in, *out, c); err != nil {
				return err
			}
		}
		if in.History != nil {
			in, out := &in.History, &out.History
			*out = make([]DockerConfigHistory, len(*in))
			for i := range *in {
				if err := DeepCopy_api_DockerConfigHistory(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		if in.OSFeatures != nil {
			in, out := &in.OSFeatures, &out.OSFeatures
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		return nil
	}
}

func DeepCopy_api_DockerImageManifest(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerImageManifest)
		out := out.(*DockerImageManifest)
		*out = *in
		if in.FSLayers != nil {
			in, out := &in.FSLayers, &out.FSLayers
			*out = make([]DockerFSLayer, len(*in))
			copy(*out, *in)
		}
		if in.History != nil {
			in, out := &in.History, &out.History
			*out = make([]DockerHistory, len(*in))
			copy(*out, *in)
		}
		if in.Layers != nil {
			in, out := &in.Layers, &out.Layers
			*out = make([]Descriptor, len(*in))
			copy(*out, *in)
		}
		return nil
	}
}

func DeepCopy_api_DockerImageReference(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerImageReference)
		out := out.(*DockerImageReference)
		*out = *in
		return nil
	}
}

func DeepCopy_api_DockerV1CompatibilityImage(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerV1CompatibilityImage)
		out := out.(*DockerV1CompatibilityImage)
		*out = *in
		out.Created = in.Created.DeepCopy()
		if err := DeepCopy_api_DockerConfig(&in.ContainerConfig, &out.ContainerConfig, c); err != nil {
			return err
		}
		if in.Config != nil {
			in, out := &in.Config, &out.Config
			*out = new(DockerConfig)
			if err := DeepCopy_api_DockerConfig(*in, *out, c); err != nil {
				return err
			}
		}
		return nil
	}
}

func DeepCopy_api_DockerV1CompatibilityImageSize(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*DockerV1CompatibilityImageSize)
		out := out.(*DockerV1CompatibilityImageSize)
		*out = *in
		return nil
	}
}

func DeepCopy_api_Image(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*Image)
		out := out.(*Image)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		if err := DeepCopy_api_DockerImage(&in.DockerImageMetadata, &out.DockerImageMetadata, c); err != nil {
			return err
		}
		if in.DockerImageLayers != nil {
			in, out := &in.DockerImageLayers, &out.DockerImageLayers
			*out = make([]ImageLayer, len(*in))
			copy(*out, *in)
		}
		if in.Signatures != nil {
			in, out := &in.Signatures, &out.Signatures
			*out = make([]ImageSignature, len(*in))
			for i := range *in {
				if err := DeepCopy_api_ImageSignature(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		if in.DockerImageSignatures != nil {
			in, out := &in.DockerImageSignatures, &out.DockerImageSignatures
			*out = make([][]byte, len(*in))
			for i := range *in {
				if (*in)[i] != nil {
					in, out := &(*in)[i], &(*out)[i]
					*out = make([]byte, len(*in))
					copy(*out, *in)
				}
			}
		}
		return nil
	}
}

func DeepCopy_api_ImageImportSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageImportSpec)
		out := out.(*ImageImportSpec)
		*out = *in
		if in.To != nil {
			in, out := &in.To, &out.To
			*out = new(pkg_api.LocalObjectReference)
			**out = **in
		}
		return nil
	}
}

func DeepCopy_api_ImageImportStatus(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageImportStatus)
		out := out.(*ImageImportStatus)
		*out = *in
		if newVal, err := c.DeepCopy(&in.Status); err != nil {
			return err
		} else {
			out.Status = *newVal.(*v1.Status)
		}
		if in.Image != nil {
			in, out := &in.Image, &out.Image
			*out = new(Image)
			if err := DeepCopy_api_Image(*in, *out, c); err != nil {
				return err
			}
		}
		return nil
	}
}

func DeepCopy_api_ImageLayer(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageLayer)
		out := out.(*ImageLayer)
		*out = *in
		return nil
	}
}

func DeepCopy_api_ImageList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageList)
		out := out.(*ImageList)
		*out = *in
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]Image, len(*in))
			for i := range *in {
				if err := DeepCopy_api_Image(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func DeepCopy_api_ImageSignature(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageSignature)
		out := out.(*ImageSignature)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		if in.Content != nil {
			in, out := &in.Content, &out.Content
			*out = make([]byte, len(*in))
			copy(*out, *in)
		}
		if in.Conditions != nil {
			in, out := &in.Conditions, &out.Conditions
			*out = make([]SignatureCondition, len(*in))
			for i := range *in {
				if err := DeepCopy_api_SignatureCondition(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		if in.SignedClaims != nil {
			in, out := &in.SignedClaims, &out.SignedClaims
			*out = make(map[string]string)
			for key, val := range *in {
				(*out)[key] = val
			}
		}
		if in.Created != nil {
			in, out := &in.Created, &out.Created
			*out = new(v1.Time)
			**out = (*in).DeepCopy()
		}
		if in.IssuedBy != nil {
			in, out := &in.IssuedBy, &out.IssuedBy
			*out = new(SignatureIssuer)
			**out = **in
		}
		if in.IssuedTo != nil {
			in, out := &in.IssuedTo, &out.IssuedTo
			*out = new(SignatureSubject)
			**out = **in
		}
		return nil
	}
}

func DeepCopy_api_ImageStream(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStream)
		out := out.(*ImageStream)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		if err := DeepCopy_api_ImageStreamSpec(&in.Spec, &out.Spec, c); err != nil {
			return err
		}
		if err := DeepCopy_api_ImageStreamStatus(&in.Status, &out.Status, c); err != nil {
			return err
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamImage(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamImage)
		out := out.(*ImageStreamImage)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		if err := DeepCopy_api_Image(&in.Image, &out.Image, c); err != nil {
			return err
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamImport(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamImport)
		out := out.(*ImageStreamImport)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		if err := DeepCopy_api_ImageStreamImportSpec(&in.Spec, &out.Spec, c); err != nil {
			return err
		}
		if err := DeepCopy_api_ImageStreamImportStatus(&in.Status, &out.Status, c); err != nil {
			return err
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamImportSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamImportSpec)
		out := out.(*ImageStreamImportSpec)
		*out = *in
		if in.Repository != nil {
			in, out := &in.Repository, &out.Repository
			*out = new(RepositoryImportSpec)
			**out = **in
		}
		if in.Images != nil {
			in, out := &in.Images, &out.Images
			*out = make([]ImageImportSpec, len(*in))
			for i := range *in {
				if err := DeepCopy_api_ImageImportSpec(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamImportStatus(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamImportStatus)
		out := out.(*ImageStreamImportStatus)
		*out = *in
		if in.Import != nil {
			in, out := &in.Import, &out.Import
			*out = new(ImageStream)
			if err := DeepCopy_api_ImageStream(*in, *out, c); err != nil {
				return err
			}
		}
		if in.Repository != nil {
			in, out := &in.Repository, &out.Repository
			*out = new(RepositoryImportStatus)
			if err := DeepCopy_api_RepositoryImportStatus(*in, *out, c); err != nil {
				return err
			}
		}
		if in.Images != nil {
			in, out := &in.Images, &out.Images
			*out = make([]ImageImportStatus, len(*in))
			for i := range *in {
				if err := DeepCopy_api_ImageImportStatus(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamList)
		out := out.(*ImageStreamList)
		*out = *in
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]ImageStream, len(*in))
			for i := range *in {
				if err := DeepCopy_api_ImageStream(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamMapping(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamMapping)
		out := out.(*ImageStreamMapping)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		if err := DeepCopy_api_Image(&in.Image, &out.Image, c); err != nil {
			return err
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamSpec)
		out := out.(*ImageStreamSpec)
		*out = *in
		if in.Tags != nil {
			in, out := &in.Tags, &out.Tags
			*out = make(map[string]TagReference)
			for key, val := range *in {
				newVal := new(TagReference)
				if err := DeepCopy_api_TagReference(&val, newVal, c); err != nil {
					return err
				}
				(*out)[key] = *newVal
			}
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamStatus(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamStatus)
		out := out.(*ImageStreamStatus)
		*out = *in
		if in.Tags != nil {
			in, out := &in.Tags, &out.Tags
			*out = make(map[string]TagEventList)
			for key, val := range *in {
				newVal := new(TagEventList)
				if err := DeepCopy_api_TagEventList(&val, newVal, c); err != nil {
					return err
				}
				(*out)[key] = *newVal
			}
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamTag(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamTag)
		out := out.(*ImageStreamTag)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*v1.ObjectMeta)
		}
		if in.Tag != nil {
			in, out := &in.Tag, &out.Tag
			*out = new(TagReference)
			if err := DeepCopy_api_TagReference(*in, *out, c); err != nil {
				return err
			}
		}
		if in.Conditions != nil {
			in, out := &in.Conditions, &out.Conditions
			*out = make([]TagEventCondition, len(*in))
			for i := range *in {
				if err := DeepCopy_api_TagEventCondition(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		if err := DeepCopy_api_Image(&in.Image, &out.Image, c); err != nil {
			return err
		}
		return nil
	}
}

func DeepCopy_api_ImageStreamTagList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ImageStreamTagList)
		out := out.(*ImageStreamTagList)
		*out = *in
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]ImageStreamTag, len(*in))
			for i := range *in {
				if err := DeepCopy_api_ImageStreamTag(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func DeepCopy_api_RepositoryImportSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*RepositoryImportSpec)
		out := out.(*RepositoryImportSpec)
		*out = *in
		return nil
	}
}

func DeepCopy_api_RepositoryImportStatus(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*RepositoryImportStatus)
		out := out.(*RepositoryImportStatus)
		*out = *in
		if newVal, err := c.DeepCopy(&in.Status); err != nil {
			return err
		} else {
			out.Status = *newVal.(*v1.Status)
		}
		if in.Images != nil {
			in, out := &in.Images, &out.Images
			*out = make([]ImageImportStatus, len(*in))
			for i := range *in {
				if err := DeepCopy_api_ImageImportStatus(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		if in.AdditionalTags != nil {
			in, out := &in.AdditionalTags, &out.AdditionalTags
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		return nil
	}
}

func DeepCopy_api_SignatureCondition(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SignatureCondition)
		out := out.(*SignatureCondition)
		*out = *in
		out.LastProbeTime = in.LastProbeTime.DeepCopy()
		out.LastTransitionTime = in.LastTransitionTime.DeepCopy()
		return nil
	}
}

func DeepCopy_api_SignatureGenericEntity(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SignatureGenericEntity)
		out := out.(*SignatureGenericEntity)
		*out = *in
		return nil
	}
}

func DeepCopy_api_SignatureIssuer(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SignatureIssuer)
		out := out.(*SignatureIssuer)
		*out = *in
		return nil
	}
}

func DeepCopy_api_SignatureSubject(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SignatureSubject)
		out := out.(*SignatureSubject)
		*out = *in
		return nil
	}
}

func DeepCopy_api_TagEvent(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*TagEvent)
		out := out.(*TagEvent)
		*out = *in
		out.Created = in.Created.DeepCopy()
		return nil
	}
}

func DeepCopy_api_TagEventCondition(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*TagEventCondition)
		out := out.(*TagEventCondition)
		*out = *in
		out.LastTransitionTime = in.LastTransitionTime.DeepCopy()
		return nil
	}
}

func DeepCopy_api_TagEventList(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*TagEventList)
		out := out.(*TagEventList)
		*out = *in
		if in.Items != nil {
			in, out := &in.Items, &out.Items
			*out = make([]TagEvent, len(*in))
			for i := range *in {
				if err := DeepCopy_api_TagEvent(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		if in.Conditions != nil {
			in, out := &in.Conditions, &out.Conditions
			*out = make([]TagEventCondition, len(*in))
			for i := range *in {
				if err := DeepCopy_api_TagEventCondition(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func DeepCopy_api_TagImportPolicy(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*TagImportPolicy)
		out := out.(*TagImportPolicy)
		*out = *in
		return nil
	}
}

func DeepCopy_api_TagReference(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*TagReference)
		out := out.(*TagReference)
		*out = *in
		if in.Annotations != nil {
			in, out := &in.Annotations, &out.Annotations
			*out = make(map[string]string)
			for key, val := range *in {
				(*out)[key] = val
			}
		}
		if in.From != nil {
			in, out := &in.From, &out.From
			*out = new(pkg_api.ObjectReference)
			**out = **in
		}
		if in.Generation != nil {
			in, out := &in.Generation, &out.Generation
			*out = new(int64)
			**out = **in
		}
		return nil
	}
}

func DeepCopy_api_TagReferencePolicy(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*TagReferencePolicy)
		out := out.(*TagReferencePolicy)
		*out = *in
		return nil
	}
}
