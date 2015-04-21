package v1beta3

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	newer "github.com/openshift/origin/pkg/template/api"
)

func init() {
	err := api.Scheme.AddConversionFuncs(
		func(in *newer.Template, out *Template, s conversion.Scope) error {
			//FIXME: DefaultConvert should not overwrite the Labels field on the
			//       the base object. This is likely a bug in the DefaultConvert
			//       code. For now, it is called before converting the labels.
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
				return err
			}
			return s.Convert(&in.ObjectLabels, &out.Labels, 0)
		},
		func(in *Template, out *newer.Template, s conversion.Scope) error {
			if err := s.Convert(&in.Labels, &out.ObjectLabels, 0); err != nil {
				return err
			}
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
	)
	if err != nil {
		panic(err)
	}
}
