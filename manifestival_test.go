package manifestival_test

import (
	"testing"

	. "github.com/jcrossley3/manifestival"
)

func TestFinding(t *testing.T) {
	f, err := NewYamlManifest("testdata/", true, nil)
	if err != nil {
		t.Errorf("NewYamlManifest() = %v, wanted no error", err)
	}

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
