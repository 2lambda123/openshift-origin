package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/user/api"
)

func init() {
	if err := kapi.Scheme.AddDefaultingFuncs(
		func(obj *Group) {
			if obj.Users == nil {
				obj.Users = []string{}
			}
		},
		func(obj *User) {
			if obj.Groups == nil {
				obj.Groups = []string{}
			}
			if obj.Identities == nil {
				obj.Identities = []string{}
			}
		},
	); err != nil {
		panic(err)
	}

	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "Group",
		oapi.GetFieldLabelConversionFunc(api.GroupToSelectableFields(&api.Group{}), nil),
	); err != nil {
		panic(err)
	}

	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "Identity",
		oapi.GetFieldLabelConversionFunc(api.IdentityToSelectableFields(&api.Identity{}), nil),
	); err != nil {
		panic(err)
	}

	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "User",
		oapi.GetFieldLabelConversionFunc(api.UserToSelectableFields(&api.User{}), nil),
	); err != nil {
		panic(err)
	}
}
