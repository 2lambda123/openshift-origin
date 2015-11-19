package admission

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// ByRestrictions is a helper to sort SCCs in order of most restrictive to least restrictive.
type ByRestrictions []*kapi.SecurityContextConstraints

func (s ByRestrictions) Len() int {
	return len(s)
}
func (s ByRestrictions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByRestrictions) Less(i, j int) bool {
	return pointValue(s[i]) < pointValue(s[j])
}

// pointValue places a value on the SCC based on the settings of the SCC that can be used
// to determine how restrictive it is.  The lower the number, the more restrictive it is.
func pointValue(constraint *kapi.SecurityContextConstraints) int {
	points := 0

	// make sure these are always valued higher than the combination of the highest strategies
	if constraint.AllowPrivilegedContainer {
		points += 20
	}
	// 9 gives us a value slightly higher than an SCC that allows run as any in both strategies since
	// we're allowing access to the host system
	if constraint.AllowHostDirVolumePlugin {
		points += 10
	}

	// strategies in order of least restrictive to most restrictive
	switch constraint.SELinuxContext.Type {
	case kapi.SELinuxStrategyRunAsAny:
		points += 4
	case kapi.SELinuxStrategyMustRunAs:
		points += 1
	}

	switch constraint.RunAsUser.Type {
	case kapi.RunAsUserStrategyRunAsAny:
		points += 4
	case kapi.RunAsUserStrategyMustRunAsNonRoot:
		points += 3
	case kapi.RunAsUserStrategyMustRunAsRange:
		points += 2
	case kapi.RunAsUserStrategyMustRunAs:
		points += 1
	}
	return points
}
