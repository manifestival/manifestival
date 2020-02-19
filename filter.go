package manifestival

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Predicate returns true if u should be included in result
type Predicate func(u *unstructured.Unstructured) bool

// Filter returns a Manifest containing only the resources for which
// *all* Predicates return true. Any changes callers make to the
// resources passed to their Predicate[s] will only be reflected in
// the returned Manifest.
func (f *Manifest) Filter(fns ...Predicate) *Manifest {
	result := *f
	result.resources = []unstructured.Unstructured{}
NEXT_RESOURCE:
	for _, spec := range f.Resources() {
		for _, pred := range fns {
			if pred != nil {
				if !pred(&spec) {
					continue NEXT_RESOURCE
				}
			}
		}
		result.resources = append(result.resources, spec)
	}
	return &result
}

func Complement(p Predicate) Predicate {
	return func(u *unstructured.Unstructured) bool {
		return !p(u)
	}
}

// JustCRDs returns only CustomResourceDefinitions
func JustCRDs(u *unstructured.Unstructured) bool {
	return u.GetKind() == "CustomResourceDefinition"
}

// NotCRDs returns no CustomResourceDefinitions
var NotCRDs = Complement(JustCRDs)

// ByLabel returns resources that contain a particular label and
// value. A value of "" denotes *ANY* value
func ByLabel(label, value string) Predicate {
	return func(u *unstructured.Unstructured) bool {
		v, ok := u.GetLabels()[label]
		if value == "" {
			return ok
		}
		return v == value
	}
}

// ByGVK returns resources of a particular GroupVersionKind
func ByGVK(gvk schema.GroupVersionKind) Predicate {
	return func(u *unstructured.Unstructured) bool {
		return u.GroupVersionKind() == gvk
	}
}
