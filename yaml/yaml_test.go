package yaml_test

import (
	"reflect"
	"testing"

	"github.com/jcrossley3/manifestival/yaml"
	"k8s.io/client-go/rest"
)

type ParseTest struct {
	path      string
	recursive bool
	resources []string
}

var parsetests = []ParseTest{
	{"testdata/", false,
		[]string{"a (/v1, Kind=Foo)", "b (/v1, Kind=Bar)"}},
	{"testdata/", true,
		[]string{"foo (/v1, Kind=A)", "bar (/v1, Kind=B)", "baz (/v1, Kind=B)", "a (/v1, Kind=Foo)", "b (/v1, Kind=Bar)"}},
	{"testdata/file.yaml", true,
		[]string{"a (/v1, Kind=Foo)", "b (/v1, Kind=Bar)"}},
	{"testdata/dir", false,
		[]string{"foo (/v1, Kind=A)", "bar (/v1, Kind=B)", "baz (/v1, Kind=B)"}},
	{"testdata/dir/a.yaml", false,
		[]string{"foo (/v1, Kind=A)"}},
	{"testdata/dir/b.yaml", false,
		[]string{"bar (/v1, Kind=B)", "baz (/v1, Kind=B)"}},
}

func TestParsing(t *testing.T) {
	for _, fixture := range parsetests {
		*yaml.Recursive = fixture.recursive
		f := yaml.NewYamlManifest(fixture.path, &rest.Config{})
		actual := f.ResourceNames()
		if !reflect.DeepEqual(actual, fixture.resources) {
			t.Errorf("Failed for '%s'; got '%s'; want '%s'", fixture.path, actual, fixture.resources)
		}
	}
}

func TestMissingFile(t *testing.T) {
	f := yaml.NewYamlManifest("testdata/missing", &rest.Config{})
	if len(f.ResourceNames()) > 0 {
		t.Error("Failed to handle missing file")
	}
}
