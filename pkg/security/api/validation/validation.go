package validation

import (
	kapivalidation "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	securityapi "github.com/openshift/origin/pkg/security/api"
)

// ValidatePodSecurityPolicySubjectReview validates PodSecurityPolicySubjectReview.
func ValidatePodSecurityPolicySubjectReview(podSecurityPolicySubjectReview *securityapi.PodSecurityPolicySubjectReview) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodSecurityPolicySubjectReviewSpec(&podSecurityPolicySubjectReview.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validatePodSecurityPolicySubjectReviewSpec(podSecurityPolicySubjectReviewSpec *securityapi.PodSecurityPolicySubjectReviewSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, kapivalidation.ValidatePodSpec(&podSecurityPolicySubjectReviewSpec.PodSpec, fldPath.Child("podSpec"))...)
	return allErrs
}

// ValidatePodSecurityPolicySelfSubjectReview validates PodSecurityPolicySelfSubjectReview.
func ValidatePodSecurityPolicySelfSubjectReview(podSecurityPolicySelfSubjectReview *securityapi.PodSecurityPolicySelfSubjectReview) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodSecurityPolicySelfSubjectReviewSpec(&podSecurityPolicySelfSubjectReview.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validatePodSecurityPolicySelfSubjectReviewSpec(podSecurityPolicySelfSubjectReviewSpec *securityapi.PodSecurityPolicySelfSubjectReviewSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, kapivalidation.ValidatePodSpec(&podSecurityPolicySelfSubjectReviewSpec.PodSpec, fldPath.Child("podSpec"))...)
	return allErrs
}

// ValidatePodSecurityPolicyReview validates PodSecurityPolicyReview.
func ValidatePodSecurityPolicyReview(podSecurityPolicyReview *securityapi.PodSecurityPolicyReview) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validatePodSecurityPolicyReviewSpec(&podSecurityPolicyReview.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validatePodSecurityPolicyReviewSpec(podSecurityPolicyReviewSpec *securityapi.PodSecurityPolicyReviewSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, kapivalidation.ValidatePodSpec(&podSecurityPolicyReviewSpec.PodSpec, fldPath.Child("podSpec"))...)
	allErrs = append(allErrs, validateServiceAccountNames(podSecurityPolicyReviewSpec.ServiceAccountNames, fldPath.Child("serviceAccountNames"))...)
	return allErrs
}

func validateServiceAccountNames(serviceAccountNames []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, sa := range serviceAccountNames {
		idxPath := fldPath.Index(i)
		switch {
		case len(sa) == 0:
			allErrs = append(allErrs, field.Invalid(idxPath, sa, ""))
		case len(sa) > 0:
			if ok, msg := kapivalidation.ValidateServiceAccountName(sa, false); !ok {
				allErrs = append(allErrs, field.Invalid(idxPath, sa, msg))
			}
		}
	}
	return allErrs
}
