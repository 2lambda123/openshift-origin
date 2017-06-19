package v1_test

import (
	"testing"

	userapi "github.com/openshift/origin/pkg/user/api"
	testutil "github.com/openshift/origin/test/util/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "Group",
		// Ensure all currently returned labels are supported
		userapi.GroupToSelectableFields(&userapi.Group{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "Identity",
		// Ensure all currently returned labels are supported
		userapi.IdentityToSelectableFields(&userapi.Identity{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"providerName", "providerUserName", "user.name", "user.uid",
	)

	testutil.CheckFieldLabelConversions(t, "v1", "User",
		// Ensure all currently returned labels are supported
		userapi.UserToSelectableFields(&userapi.User{}),
	)
}
