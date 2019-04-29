package manifestival_test

import (
	"os"
	"testing"

	. "github.com/jcrossley3/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTransform(t *testing.T) {
	f, err := NewManifest("testdata/tree", true, nil)
	if err != nil {
		t.Errorf("NewManifest() = %v, wanted no error", err)
	}

	actual := f.Resources
	if len(actual) != 5 {
		t.Errorf("Failed to read all resources: %s", actual)
	}
	f.Transform(func(u *unstructured.Unstructured) *unstructured.Unstructured {
		u.SetResourceVersion("69")
		if u.GetKind() == "B" {
			return u
		}
		return nil
	})
	transformed := f.Resources
	if len(transformed) != 2 {
		t.Errorf("Failed to transform by kind: %s", transformed)
	}
	// Ensure all transformed have a version and B kind
	for _, spec := range transformed {
		if spec.GetResourceVersion() != "69" || spec.GetKind() != "B" {
			t.Errorf("The transform didn't work: %s", transformed)
		}
	}
	// Ensure we didn't change the previous resources
	for _, spec := range actual {
		if spec.GetResourceVersion() != "" {
			t.Errorf("The transform shouldn't affect previous resources: %s", actual)
		}
	}
}

func TestTransformCombo(t *testing.T) {
	f, err := NewManifest("testdata/tree", true, nil)
	if err != nil {
		t.Errorf("NewManifest() = %v, wanted no error", err)
	}
	if len(f.Resources) != 5 {
		t.Errorf("Failed to read all resources: %s", f.Resources)
	}
	fn1 := func(u *unstructured.Unstructured) *unstructured.Unstructured {
		if u.GetKind() == "B" {
			return u
		}
		return nil
	}
	fn2 := func(u *unstructured.Unstructured) *unstructured.Unstructured {
		if u.GetName() == "bar" {
			return u
		}
		return nil
	}
	x := f.Transform(fn1, fn2).Resources
	if len(x) != 1 || x[0].GetName() != "bar" || x[0].GetKind() != "B" {
		t.Errorf("Failed to transform by combo: %s", x)
	}
}

func TestInjectNamespace(t *testing.T) {
	assert := func(u unstructured.Unstructured, expected string) {
		v, _, _ := unstructured.NestedSlice(u.Object, "subjects")
		ns := v[0].(map[string]interface{})["namespace"]
		if ns != expected {
			t.Errorf("Expected '%s', got '%s'", expected, ns)
		}
	}
	f, _ := NewManifest("testdata/crb.yaml", true, nil)
	resources := f.Resources
	if len(resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(resources))
	}
	x := f.Transform(InjectNamespace("foo")).Resources
	resources = f.Resources
	if len(resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(resources))
	}
	assert(x[0], "foo")
	os.Setenv("FOO", "foo")
	x = f.Transform(InjectNamespace("$FOO")).Resources
	assert(x[0], "foo")
}
