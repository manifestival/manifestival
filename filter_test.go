package manifestival_test

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/manifestival/manifestival"
)

func True(_ *unstructured.Unstructured) bool {
	return true
}

var False = Not(True)

func TestFilter(t *testing.T) {
	manifest, _ := NewManifest("testdata/k-s-v0.12.1.yaml")
	tests := []struct {
		name       string
		predicates []Predicate
		count      int
	}{{
		name:       "Nothing",
		predicates: []Predicate{Nothing},
		count:      0,
	}, {
		name:       "Everything",
		predicates: []Predicate{Everything},
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
		name:       "Resources match for one label",
		predicates: []Predicate{ByLabels(map[string]string{"networking.knative.dev/ingress-provider": "istio"})},
		count:      5,
	}, {
		name:       "Resources match for any of the labels",
		predicates: []Predicate{ByLabels(map[string]string{"networking.knative.dev/ingress-provider": "istio", "autoscaling.knative.dev/metric-provider": "custom-metrics"})},
		count:      10,
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
		name:       "Any, first true then false",
		predicates: []Predicate{Any(True, False)},
		count:      55,
	}, {
		name:       "Any, first false then true",
		predicates: []Predicate{Any(False, True)},
		count:      55,
	}, {
		name:       "Any, both true",
		predicates: []Predicate{Any(True, True)},
		count:      55,
	}, {
		name:       "Any, both false",
		predicates: []Predicate{Any(False, False)},
		count:      0,
	}, {
		name:       "Not true",
		predicates: []Predicate{Not(True)},
		count:      0,
	}, {
		name:       "Not false",
		predicates: []Predicate{Not(False)},
		count:      55,
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
	m, _ := NewManifest("testdata/k-s-v0.12.1.yaml")
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

func TestConvertFilter(t *testing.T) {
	manifest, _ := NewManifest("testdata/k-s-v0.12.1.yaml")
	filter := func(u *unstructured.Unstructured) bool {
		// Another abuse of Predicate, to ensure Convert works
		if u.GetKind() == "ConfigMap" {
			cm := &v1.ConfigMap{}
			scheme.Scheme.Convert(u, cm, nil)
			cm.Data["foo"] = "bar"
			scheme.Scheme.Convert(cm, u, nil)
			return true
		}
		return false
	}
	actual := manifest.Filter(filter)
	if 0 == len(actual.Resources()) {
		t.Error("Not enough ConfigMaps!")
	}
	for _, u := range actual.Resources() {
		cm := &v1.ConfigMap{}
		if err := scheme.Scheme.Convert(&u, cm, nil); err != nil {
			t.Error(err)
		}
		if cm.Data["foo"] != "bar" {
			t.Error("Data not there")
		}
	}
}

func TestInFilter(t *testing.T) {
	eleven, _ := NewManifest("testdata/k-s-v0.11.0.yaml")
	twelve, _ := NewManifest("testdata/k-s-v0.12.1.yaml")
	new := twelve.Filter(Not(In(eleven)))
	assert(t, len(new.Resources()), 1)

	// now verify version doesn't matter
	crd := &unstructured.Unstructured{}
	crd.SetAPIVersion("apiextensions.k8s.io/v1beta1")
	crd.SetKind("CustomResourceDefinition")
	crd.SetName("foo")
	crdv1 := crd.DeepCopy()
	crdv1.SetAPIVersion("apiextensions.k8s.io/v1")

	m, _ := ManifestFrom(Slice([]unstructured.Unstructured{*crd}))
	mv1, _ := ManifestFrom(Slice([]unstructured.Unstructured{*crdv1}))
	assert(t, len(m.Filter(Not(In(mv1))).Resources()), 0)
	crdv1.SetName("bar")
	assert(t, len(m.Filter(Not(In(mv1))).Resources()), 1)
}

func TestAnnotations(t *testing.T) {
	manifest, _ := NewManifest("testdata/tree/file.yaml")
	tests := []struct {
		name       string
		predicates []Predicate
		count      int
	}{{
		name:       "No matches for specific annotation",
		predicates: []Predicate{ByAnnotation("foo", "bar")},
		count:      0,
	}, {
		name:       "No matches for any annotation",
		predicates: []Predicate{ByAnnotation("missing", "")},
		count:      0,
	}, {
		name:       "Annotation has any value",
		predicates: []Predicate{ByAnnotation("foo", "")},
		count:      2,
	}, {
		name:       "Annotation has specific value",
		predicates: []Predicate{ByAnnotation("foo", "true")},
		count:      1,
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
