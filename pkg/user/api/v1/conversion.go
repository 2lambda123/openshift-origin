package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "Group",
		oapi.GetFieldLabelConversionFunc(userapi.GroupToSelectableFields(&userapi.Group{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "Identity",
		oapi.GetFieldLabelConversionFunc(userapi.IdentityToSelectableFields(&userapi.Identity{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "User",
		oapi.GetFieldLabelConversionFunc(userapi.UserToSelectableFields(&userapi.User{}), nil),
	); err != nil {
		return err
	}
	return nil
}
