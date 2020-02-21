package manifestival_test

import (
	"testing"

	. "github.com/manifestival/manifestival"
)

func TestManifestChaining(t *testing.T) {
	const expected = 6
	manifest, _ := NewManifest("testdata/knative-serving.yaml")
	// Filter->Transform->Resources
	deployments, _ := manifest.Filter(ByKind("Deployment")).Transform(InjectNamespace("foo"))
	if len(deployments.Resources()) != expected {
		t.Errorf("F->T->R, expected %d deployments, got %d", expected, len(deployments.Resources()))
	}
	// Filter->Resources
	count := len(manifest.Filter(ByKind("Deployment")).Resources())
	if count != expected {
		t.Errorf("F->R, expected %d deployments, got %d", expected, count)
	}
}
