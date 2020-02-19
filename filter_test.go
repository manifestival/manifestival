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

func False(_ *unstructured.Unstructured) bool {
	return false
}

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
		predicates: []Predicate{NotCRDs},
		count:      45,
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
