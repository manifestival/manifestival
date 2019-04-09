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
		*yaml.Recursive = fixture.recursive
		f := yaml.NewYamlManifest(fixture.path, &rest.Config{}, "")
		actual := f.DeepCopyResources()
		for i, spec := range actual {
			if spec.GetName() != fixture.resources[i] {
				t.Errorf("Failed for '%s'; got '%s'; want '%s'", fixture.path, actual, fixture.resources)
			}
		}
	}
}

func TestMissingFile(t *testing.T) {
	f := yaml.NewYamlManifest("testdata/missing", &rest.Config{}, "")
	if len(f.ResourceNames()) > 0 {
		t.Error("Failed to handle missing file")
	}
}

func TestFilter(t *testing.T) {
	*yaml.Recursive = true
	f := yaml.NewYamlManifest("testdata/", &rest.Config{}, "")
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

func TestFinding(t *testing.T) {
	*yaml.Recursive = true
	f := yaml.NewYamlManifest("testdata/", &rest.Config{}, "fubar")
	actual := f.Find("v1", "A", "foo")
	if actual == nil {
		t.Error("Failed to find resource")
	}
	if actual.GetNamespace() != "fubar" {
		t.Errorf("Resource has wrong namespace: %s", actual)
	}
}
