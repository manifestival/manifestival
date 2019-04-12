package manifestival_test

import (
	"testing"

	. "github.com/jcrossley3/manifestival"
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
	{"https://raw.githubusercontent.com/jcrossley3/manifestival/master/testdata/file.yaml", true, []string{"a", "b"}},
}

func TestParsing(t *testing.T) {
	for _, fixture := range parsetests {
		actual := Parse(fixture.path, fixture.recursive)
		for i, spec := range actual {
			if spec.GetName() != fixture.resources[i] {
				t.Errorf("Failed for '%s'; got '%s'; want '%s'", fixture.path, actual, fixture.resources)
			}
		}
	}
}

func TestMissingFile(t *testing.T) {
	f := Parse("testdata/missing", false)
	if len(f) > 0 {
		t.Error("Failed to handle missing file")
	}
}

func TestFinding(t *testing.T) {
	f := NewYamlManifest("testdata/", true, &rest.Config{})
	f.Filter(ByNamespace("fubar"))
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
