package manifestival_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/manifestival/manifestival"
)

func True(_ *unstructured.Unstructured) bool {
	return true
}

var False = None(True)

func TestFilter(t *testing.T) {
	manifest, _ := NewManifest("testdata/knative-serving.yaml")
	tests := []struct {
		name       string
		predicates []Predicate
		count      int
	}{{
		name:       "No predicates",
		predicates: []Predicate{},
		count:      55,
	}, {
		name:       "No matches for label",
		predicates: []Predicate{ByLabel("foo", "bar")},
		count:      0,
	}, {
		name:       "Label has any value",
		predicates: []Predicate{ByLabel("istio-injection", "")},
		count:      1,
	}, {
		name:       "Label has specific value",
		predicates: []Predicate{ByLabel("serving.knative.dev/release", "v0.12.1")},
		count:      54,
	}, {
		name:       "First true then false",
		predicates: []Predicate{True, False},
		count:      0,
	}, {
		name:       "First false then true",
		predicates: []Predicate{False, True},
		count:      0,
	}, {
		name:       "Both true",
		predicates: []Predicate{True, True},
		count:      55,
	}, {
		name:       "Both false",
		predicates: []Predicate{False, False},
		count:      0,
	}, {
		name:       "One match By GVK",
		predicates: []Predicate{ByGVK(schema.GroupVersionKind{Kind: "Namespace", Version: "v1"})},
		count:      1,
	}, {
		name:       "Without CRD's",
		predicates: []Predicate{NoCRDs},
		count:      45,
	}, {
		name:       "Only CRD's",
		predicates: []Predicate{CRDs},
		count:      10,
	}, {
		name:       "No CRD's",
		predicates: []Predicate{NoCRDs, CRDs},
		count:      0,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := manifest.Filter(test.predicates...)
			count := len(actual.Resources())
			if count != test.count {
				t.Errorf("wanted %v, got %v", test.count, count)
			}
		})
	}
}

func TestFilterMutation(t *testing.T) {
	m, _ := NewManifest("testdata/knative-serving.yaml")
	bobs := m.Filter(func(u *unstructured.Unstructured) bool {
		// This is an abuse of a Predicate, but allowed for those
		// times you'd prefer not to deal with the multi-valued result
		// of Transform
		u.SetName("bob")
		return true
	})

	if 0 != len(m.Filter(ByName("bob")).Resources()) {
		t.Error("Even one bob is too many")
	}
	if 55 != len(bobs.Filter(ByName("bob")).Resources()) {
		t.Error("Not every one is bob")
	}
}
