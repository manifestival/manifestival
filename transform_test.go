package manifestival_test

import (
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/manifestival/manifestival"
)

func TestTransform(t *testing.T) {
	f, _ := ManifestFrom(Recursive("testdata/tree"))
	actual := f.Resources()
	if len(actual) != 5 {
		t.Errorf("Failed to read all resources: %s", actual)
	}
	f, _ = f.Transform(func(u *unstructured.Unstructured) error {
		if u.GetKind() == "B" {
			u.SetResourceVersion("69")
		}
		return nil
	})
	transformed := f.Resources()
	// Ensure all transformed have a version and B kind
	for _, spec := range transformed {
		if spec.GetResourceVersion() != "69" && spec.GetKind() == "B" {
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
	f, err := ManifestFrom(Recursive("testdata/tree"))
	if len(f.Resources()) != 5 {
		t.Errorf("Failed to read all resources: %s", f.Resources())
	}
	fn1 := func(u *unstructured.Unstructured) error {
		if u.GetKind() == "B" {
			u.SetResourceVersion("69")
		}
		return nil
	}
	fn2 := func(u *unstructured.Unstructured) error {
		if u.GetName() == "bar" {
			u.SetResourceVersion("42")
		}
		return nil
	}
	if f, err = f.Transform(fn1, fn2); err != nil {
		t.Error(err)
	}
	for _, x := range f.Resources() {
		if x.GetName() == "bar" && x.GetResourceVersion() != "42" {
			t.Errorf("Failed to transform bar by combo: %s", x)
		}
		if x.GetName() == "B" && x.GetResourceVersion() != "69" {
			t.Errorf("Failed to transform B by combo: %s", x)
		}
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
	f, err := NewManifest("testdata/crb.yaml")
	if f, err = f.Transform(InjectNamespace("foo")); err != nil {
		t.Error(err)
	}
	if f.Resources()[0].GetName() != "foo" {
		t.Errorf("Expected namespace name to be foo, got %s", f.Resources()[0].GetName())
	}
	assert(f.Resources()[1], "foo")
	os.Setenv("FOO", "food")
	if f, err = f.Transform(InjectNamespace("$FOO")); err != nil {
		t.Error(err)
	}
	if f.Resources()[0].GetName() != "food" {
		t.Errorf("Expected namespace name to be food, got %s", f.Resources()[0].GetName())
	}
	assert(f.Resources()[1], "food")
}

func TestInjectNamespaceRoleBinding(t *testing.T) {
	assert := func(u unstructured.Unstructured, expected string) {
		v, _, _ := unstructured.NestedSlice(u.Object, "subjects")
		ns := v[0].(map[string]interface{})["namespace"]
		if ns != expected {
			t.Errorf("Expected '%s', got '%s'", expected, ns)
		}
		ns = u.GetNamespace()
		if ns != expected {
			t.Errorf("Expected '%s', got '%s'", expected, ns)
		}
	}
	f, err := NewManifest("testdata/rb.yaml")
	if f, err = f.Transform(InjectNamespace("foo")); err != nil {
		t.Error(err)
	}
	if f.Resources()[0].GetName() != "foo" {
		t.Errorf("Expected namespace name to be foo, got %s", f.Resources()[0].GetName())
	}
	assert(f.Resources()[1], "foo")
	os.Setenv("FOO", "food")
	if f, err = f.Transform(InjectNamespace("$FOO")); err != nil {
		t.Error(err)
	}
	if f.Resources()[0].GetName() != "food" {
		t.Errorf("Expected namespace name to be food, got %s", f.Resources()[0].GetName())
	}
	assert(f.Resources()[1], "food")
}

func TestInjectNamespaceWebhook(t *testing.T) {
	assert := func(u unstructured.Unstructured, expected string) {
		v, _, _ := unstructured.NestedSlice(u.Object, "webhooks")
		ns, _, err := unstructured.NestedString(v[0].(map[string]interface{}), "clientConfig", "service", "namespace")
		if err != nil {
			t.Errorf("Failed to find `clientConfig.service.namespace`: %v", err)
		}
		if ns != expected {
			t.Errorf("Expected %q, got %q", expected, ns)
		}
	}

	f, err := NewManifest("testdata/hooks.yaml")
	if f, err = f.Transform(InjectNamespace("foo")); err != nil {
		t.Error(err)
	}
	assert(f.Resources()[0], "foo")
	os.Setenv("FOO", "food")
	if f, err = f.Transform(InjectNamespace("$FOO")); err != nil {
		t.Error(err)
	}
	assert(f.Resources()[0], "food")
}

func TestInjectNamespaceAPIService(t *testing.T) {
	assert := func(u unstructured.Unstructured, expected string) {
		ns, _, err := unstructured.NestedString(u.Object, "spec", "service", "namespace")
		if err != nil {
			t.Errorf("Failed to find `service.namespace`: %v", err)
		}
		if ns != expected {
			t.Errorf("Expected %q, got %q", expected, ns)
		}
	}

	f, err := NewManifest("testdata/apiservice.yaml")
	if f, err = f.Transform(InjectNamespace("foo")); err != nil {
		t.Error(err)
	}
	assert(f.Resources()[0], "foo")
	os.Setenv("FOO", "food")
	if f, err = f.Transform(InjectNamespace("$FOO")); err != nil {
		t.Error(err)
	}
	assert(f.Resources()[0], "food")
}

func TestInjectNamespaceCRD(t *testing.T) {
	assert := func(u unstructured.Unstructured, expected string) {
		var ns string
		var err error
		if u.GroupVersionKind().Version == "v1" {
			ns, _, err = unstructured.NestedString(u.Object, "spec", "conversion", "webhook", "clientConfig", "service", "namespace")
		} else {
			ns, _, err = unstructured.NestedString(u.Object, "spec", "conversion", "webhookClientConfig", "service", "namespace")
		}
		if err != nil {
			t.Errorf("Failed to find `service.namespace`: %v", err)
		}
		if ns != expected {
			t.Errorf("Expected %q, got %q", expected, ns)
		}
	}
	f, err := NewManifest("testdata/crd.yaml")
	if f, err = f.Transform(InjectNamespace("foo")); err != nil {
		t.Error(err)
	}
	for _, resource := range f.Resources() {
		assert(resource, "foo")
	}
	os.Setenv("FOO", "food")
	if f, err = f.Transform(InjectNamespace("$FOO")); err != nil {
		t.Error(err)
	}
	assert(f.Resources()[0], "food")
}

func TestConvertTransform(t *testing.T) {
	manifest, _ := NewManifest("testdata/k-s-v0.12.1.yaml")
	transformer := func(u *unstructured.Unstructured) error {
		if u.GetKind() == "ConfigMap" {
			cm := &v1.ConfigMap{}
			if err := scheme.Scheme.Convert(u, cm, nil); err != nil {
				return err
			}
			cm.Data["foo"] = "bar"
			return scheme.Scheme.Convert(cm, u, nil)
		}
		return nil
	}
	actual, err := manifest.Transform(transformer)
	if err != nil {
		t.Error(err)
	}
	configmaps := actual.Filter(ByKind("ConfigMap")).Resources()
	if len(configmaps) == 0 {
		t.Error("Not enough configmaps")
	}
	for _, u := range configmaps {
		cm := &v1.ConfigMap{}
		if err := scheme.Scheme.Convert(&u, cm, nil); err != nil {
			t.Error(err)
		}
		if cm.Data["foo"] != "bar" {
			t.Error("Data not there")
		}
	}
}
