package manifestival_test

import (
	"testing"

	. "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDiff(t *testing.T) {
	client := testClient()
	old, _ := NewManifest("testdata/k-s-v0.11.0.yaml", UseClient(client))
	old.Apply()
	new, _ := NewManifest("testdata/k-s-v0.12.1.yaml", UseClient(client))
	diffs, err := new.Filter(ignoreReleaseLabel(old)).Diff()
	if err != nil {
		t.Error(err)
	}
	expected := 21
	if len(diffs) != expected {
		t.Errorf("Expected %d diffs, got %d", expected, len(diffs))
	}
	// buf, _ := yaml.Marshal(diffs)
	// fmt.Println(string(buf))
}

func ignoreReleaseLabel(old Manifest) Predicate {
	const key = "serving.knative.dev/release"
	return func(u *unstructured.Unstructured) bool {
		found := old.Filter(ByGVK(u.GroupVersionKind()), ByName(u.GetName())).Resources()
		if len(found) == 0 {
			return true
		}
		labels := u.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		if _, ok := labels[key]; ok {
			labels[key] = found[0].GetLabels()[key]
			u.SetLabels(labels)
		}
		return true
	}
}
