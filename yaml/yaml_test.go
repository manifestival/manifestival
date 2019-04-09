package yaml_test

import (
	"testing"

	"github.com/jcrossley3/manifestival/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

type ParseTest struct {
	path      string
	recursive bool
	resources []string
}

var parsetests = []ParseTest{
	{"testdata/", false, []string{"a", "b"}},
	{"testdata/", true, []string{"foo", "bar", "baz", "a", "b"}},
	{"testdata/file.yaml", true, []string{"a", "b"}},
	{"testdata/dir", false, []string{"foo", "bar", "baz"}},
	{"testdata/dir/a.yaml", false, []string{"foo"}},
	{"testdata/dir/b.yaml", false, []string{"bar", "baz"}},
}

func TestParsing(t *testing.T) {
	for _, fixture := range parsetests {
		f := yaml.NewYamlManifest(fixture.path, fixture.recursive, &rest.Config{})
		actual := f.DeepCopyResources()
		for i, spec := range actual {
			if spec.GetName() != fixture.resources[i] {
				t.Errorf("Failed for '%s'; got '%s'; want '%s'", fixture.path, actual, fixture.resources)
			}
		}
	}
}

func TestMissingFile(t *testing.T) {
	f := yaml.NewYamlManifest("testdata/missing", false, &rest.Config{})
	if len(f.ResourceNames()) > 0 {
		t.Error("Failed to handle missing file")
	}
}

func TestFilter(t *testing.T) {
	f := yaml.NewYamlManifest("testdata/", true, &rest.Config{})
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
	f := yaml.NewYamlManifest("testdata/", true, &rest.Config{})
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

func TestFinding(t *testing.T) {
	f := yaml.NewYamlManifest("testdata/", true, &rest.Config{})
	f.Filter(yaml.ByNamespace("fubar"))
	actual := f.Find("v1", "A", "foo")
	if actual == nil {
		t.Error("Failed to find resource")
	}
	if actual.GetNamespace() != "fubar" {
		t.Errorf("Resource has wrong namespace: %s", actual)
	}
	if f.Find("NO", "NO", "NO") != nil {
		t.Error("Missing resource found")
	}
}
