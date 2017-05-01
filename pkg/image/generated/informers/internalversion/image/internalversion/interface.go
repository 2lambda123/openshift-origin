// This file was automatically generated by informer-gen

package internalversion

import (
	internalinterfaces "github.com/openshift/origin/pkg/image/generated/informers/internalversion/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// Images returns a ImageInformer.
	Images() ImageInformer
}

type version struct {
	internalinterfaces.SharedInformerFactory
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory) Interface {
	return &version{f}
}

// Images returns a ImageInformer.
func (v *version) Images() ImageInformer {
	return &imageInformer{factory: v.SharedInformerFactory}
}
