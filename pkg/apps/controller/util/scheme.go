package util

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/origin/pkg/api/legacy"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsv1helpers "github.com/openshift/origin/pkg/apps/apis/apps/v1"
)

var (
	// for decoding, we want to be tolerant of groupified and non-groupified
	annotationDecodingScheme = runtime.NewScheme()
	annotationDecoder        runtime.Decoder

	// for encoding, we want to be strict on groupified
	annotationEncodingScheme = runtime.NewScheme()
	annotationEncoder        runtime.Encoder
)

func init() {
	legacy.InstallLegacyApps(annotationDecodingScheme)
	// TODO eventually we shouldn't deal in internal versions, but for now decode into one.
	utilruntime.Must(appsv1helpers.Install(annotationDecodingScheme))
	annotationDecoderCodecFactory := serializer.NewCodecFactory(annotationDecodingScheme)
	annotationDecoder = annotationDecoderCodecFactory.UniversalDecoder(appsapi.SchemeGroupVersion)

	// TODO eventually we shouldn't deal in internal versions, but for now decode into one.
	utilruntime.Must(appsv1helpers.Install(annotationEncodingScheme))
	annotationEncoderCodecFactory := serializer.NewCodecFactory(annotationEncodingScheme)
	annotationEncoder = annotationEncoderCodecFactory.LegacyCodec(appsv1.GroupVersion)
}
