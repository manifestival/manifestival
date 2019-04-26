package manifestival_test

import (
	"os"
	"testing"

	. "github.com/jcrossley3/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFilter(t *testing.T) {
	f, err := NewYamlManifest("testdata/tree", true, nil)
	if err != nil {
		t.Errorf("NewYamlManifest() = %v, wanted no error", err)
	}

	actual := f.DeepCopyResources()
	if len(actual) != 5 {
		t.Errorf("Failed to read all resources: %s", actual)
	}
	f.Filter(func(u *unstructured.Unstructured) bool {
		u.SetResourceVersion("69")
		return u.GetKind() == "B"
	})
	filtered := f.DeepCopyResources()
	if len(filtered) != 2 {
		t.Errorf("Failed to filter by kind: %s", filtered)
	}
	// Ensure all filtered have a version and B kind
	for _, spec := range filtered {
		if spec.GetResourceVersion() != "69" || spec.GetKind() != "B" {
			t.Errorf("The filter didn't work: %s", filtered)
		}
	}
	// Ensure we didn't change the previous resources
	for _, spec := range actual {
		if spec.GetResourceVersion() != "" {
			t.Errorf("The filter shouldn't affect previous resources: %s", actual)
		}
	}
}

func TestFilterCombo(t *testing.T) {
	f, err := NewYamlManifest("testdata/tree", true, nil)
	if err != nil {
		t.Errorf("NewYamlManifest() = %v, wanted no error", err)
	}

	actual := f.DeepCopyResources()
	if len(actual) != 5 {
		t.Errorf("Failed to read all resources: %s", actual)
	}
	fn1 := func(u *unstructured.Unstructured) bool {
		return u.GetKind() == "B"
	}
	fn2 := func(u *unstructured.Unstructured) bool {
		return u.GetName() == "bar"
	}
	x := f.Filter(fn1, fn2).DeepCopyResources()
	if len(x) != 1 || x[0].GetName() != "bar" || x[0].GetKind() != "B" {
		t.Errorf("Failed to filter by combo: %s", x)
	}
}

func TestByNamespace(t *testing.T) {
	assert := func(u unstructured.Unstructured, expected string) {
		v, _, _ := unstructured.NestedSlice(u.Object, "subjects")
		ns := v[0].(map[string]interface{})["namespace"]
		if ns != expected {
			t.Errorf("Expected '%s', got '%s'", expected, ns)
		}
	}
	f, _ := NewYamlManifest("testdata/crb.yaml", true, nil)
	x := f.Filter(ByNamespace("foo")).DeepCopyResources()
	assert(x[0], "foo")
	os.Setenv("FOO", "foo")
	x = f.Filter(ByNamespace("$FOO")).DeepCopyResources()
	assert(x[0], "foo")
}
