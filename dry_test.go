package manifestival_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"testing"

	. "github.com/manifestival/manifestival"
	"github.com/manifestival/manifestival/fake"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDryRun(t *testing.T) {
	client := fake.New()
	ctx := context.Background()
	current, _ := NewManifest("testdata/dry/current.yaml", UseClient(client))
	current.Apply(ctx)
	modified, _ := NewManifest("testdata/dry/modified.yaml", UseClient(client))
	diffs, err := modified.DryRun(ctx)
	if err != nil {
		t.Error(err)
	}
	actual, _ := json.MarshalIndent(diffs, "", "  ")
	expect, _ := ioutil.ReadFile("testdata/dry/expected.json")
	expect = bytes.TrimSpace(expect)
	if !bytes.Equal(actual, expect) {
		t.Errorf("Wrong patch! Expected:\n%s\nGot:\n%s", string(expect), string(actual))
	}
}

func TestNothingChanged(t *testing.T) {
	client := fake.New()
	ctx := context.Background()
	current, _ := NewManifest("testdata/dry/current.yaml", UseClient(client))
	current.Apply(ctx)
	diffs, err := current.DryRun(ctx)
	if err != nil {
		t.Error(err)
	}
	if len(diffs) > 0 {
		t.Errorf("Nothing should've changed!")
	}
}

func TestKnativeUpgrade(t *testing.T) {
	client := fake.New()
	ctx := context.Background()
	old, _ := NewManifest("testdata/k-s-v0.11.0.yaml", UseClient(client))
	old.Apply(ctx)
	new, _ := NewManifest("testdata/k-s-v0.12.1.yaml", UseClient(client))
	// Transform to omit version label noise
	unversioned, _ := new.Transform(ignoreReleaseLabel(old))
	diffs, err := unversioned.DryRun(ctx)
	if err != nil {
		t.Error(err)
	}
	expected := 21
	if len(diffs) != expected {
		t.Errorf("Expected %d diffs, got %d", expected, len(diffs))
	}
	// Now do unfiltered
	diffs, err = new.DryRun(ctx)
	if err != nil {
		t.Error(err)
	}
	expected = len(old.Resources())
	if len(diffs) != expected {
		t.Errorf("Expected %d diffs, got %d", expected, len(diffs))
	}
}

func ignoreReleaseLabel(old Manifest) Transformer {
	const key = "serving.knative.dev/release"
	return func(u *unstructured.Unstructured) error {
		found := old.Filter(ByGVK(u.GroupVersionKind()), ByName(u.GetName())).Resources()
		if len(found) == 0 {
			return nil
		}
		labels := u.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		if _, ok := labels[key]; ok {
			labels[key] = found[0].GetLabels()[key]
			u.SetLabels(labels)
		}
		return nil
	}
}
