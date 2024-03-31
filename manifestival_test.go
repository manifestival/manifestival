package manifestival_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	logr "github.com/go-logr/logr/testing"
	. "github.com/manifestival/manifestival"
	"github.com/manifestival/manifestival/fake"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestPortUpdates(t *testing.T) {
	ctx := context.Background()
	specBytes, _ := ioutil.ReadFile("testdata/kourier/deployment-spec.json")
	editedBytes, _ := ioutil.ReadFile("testdata/kourier/deployment-edited.json")
	spec := &unstructured.Unstructured{}
	edited := &unstructured.Unstructured{}
	if err := spec.UnmarshalJSON(specBytes); err != nil {
		t.Error(err)
	}
	if err := edited.UnmarshalJSON(editedBytes); err != nil {
		t.Error(err)
	}
	c := fake.New(edited)
	c.Stubs.Update = func(ctx context.Context, obj *unstructured.Unstructured) error {
		d := &appsv1.Deployment{}
		scheme.Scheme.Convert(obj, d, nil)
		names := sets.String{}
		for _, port := range d.Spec.Template.Spec.Containers[0].Ports {
			// raise error if >1 ports with same name
			if names.Has(port.Name) {
				return errors.NewInvalid(obj.GroupVersionKind().GroupKind(), obj.GetName(), nil)
			} else {
				names.Insert(port.Name)
			}
		}
		return nil
	}
	manifest, _ := ManifestFrom(Slice([]unstructured.Unstructured{*spec}), UseClient(c))
	if err := manifest.Apply(ctx, Overwrite(false)); err == nil {
		t.Error("Should have received an invalid error")
	}
	if err := manifest.Apply(ctx); err != nil {
		t.Error(err)
	}
}

func TestLastAppliedAnnotation(t *testing.T) {
	ctx := context.Background()
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Data: map[string]string{"foo": "bar"},
	}
	u := unstructured.Unstructured{}
	scheme.Scheme.Convert(&cm, &u, nil)
	ub, _ := u.MarshalJSON()
	expected := string(ub)
	// Seed the fake client with a different configmap
	cm.Data["foo"] = "baz"
	cm.Data["bizz"] = "buzz"
	client := fake.New(&cm)
	// Use the unstructured for our manifest
	m, _ := ManifestFrom(Slice([]unstructured.Unstructured{u}), UseClient(client))
	if err := m.Apply(ctx); err != nil {
		t.Error(err)
	}
	x, _ := m.Client.Get(ctx, &u)
	actual := x.GetAnnotations()[v1.LastAppliedConfigAnnotation]
	assert(t, actual, expected)
}

func TestLastAppliedAnnotationAlt(t *testing.T) {
	ctx := context.Background()
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Data: map[string]string{"foo": "bar"},
	}
	u := unstructured.Unstructured{}
	scheme.Scheme.Convert(&cm, &u, nil)
	ub, _ := u.MarshalJSON()
	expected := string(ub)
	// Seed the fake client with a different configmap
	cm.Data["foo"] = "baz"
	cm.Data["bizz"] = "buzz"
	client := fake.New(&cm)
	annotationName := "test.example.com/last-applied-configuration"
	// Use the unstructured for our manifest
	m, _ := ManifestFrom(Slice([]unstructured.Unstructured{u}), UseClient(client), UseLastAppliedConfigAnnotation(annotationName))
	if err := m.Apply(ctx); err != nil {
		t.Error(err)
	}
	x, _ := m.Client.Get(ctx, &u)
	actual := x.GetAnnotations()[annotationName]
	assert(t, actual, expected)
	actual = x.GetAnnotations()[v1.LastAppliedConfigAnnotation]
	assert(t, actual, "")
}

func TestMethodChaining(t *testing.T) {
	ctx := context.Background()
	const expected = 6
	const kind = "Deployment"
	const name = "controller"
	manifest, _ := NewManifest("testdata/k-s-v0.12.1.yaml", UseClient(fake.New()), UseLogger(logr.NewTestLogger(t)))
	// Filter->Transform->Resources
	deployments, _ := manifest.Filter(ByKind(kind)).Transform(InjectNamespace("foo"))
	assert(t, len(deployments.Resources()), expected)
	// Filter->Apply
	if err := manifest.Filter(ByKind(kind)).Apply(ctx); err != nil {
		t.Error(err, "Expected deployments to be applied")
	}
	// Filter->Resources
	u := manifest.Filter(ByKind(kind), ByName(name)).Resources()[0]
	if _, err := manifest.Client.Get(ctx, &u); err != nil {
		t.Error(err, "Expected controller deployment to be created")
	}
	// Filter->Delete
	if err := manifest.Filter(ByKind(kind), ByName(name)).Delete(ctx); err != nil {
		t.Error(err)
	}
	if _, err := manifest.Client.Get(ctx, &u); !errors.IsNotFound(err) {
		t.Error(err, "Expected controller deployment to be deleted")
	}
}

func TestOverwriteApply(t *testing.T) {
	current := bytes.NewReader([]byte(`
apiVersion: v1
kind: ComponentStatus
metadata:
  name: test
conditions:
- type: foo
  status: "True"
`))
	config := bytes.NewReader([]byte(`
apiVersion: v1
kind: ComponentStatus
metadata:
  name: test
conditions:
- type: bar
  status: "False"
`))
	tests := []struct {
		name      string
		overwrite bool
		expected  int
	}{{
		name:      "Merge patch",
		overwrite: false,
		expected:  2,
	}, {
		name:      "Overwrite",
		overwrite: true,
		expected:  1,
	}}
	setup, _ := ManifestFrom(Reader(current))
	original := setup.Resources()[0]
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.New(&original)
			tgt, _ := ManifestFrom(Reader(config), UseClient(client))
			tgt.Apply(ctx, Overwrite(test.overwrite))
			obj, _ := tgt.Client.Get(ctx, &original)
			actual, _, _ := unstructured.NestedSlice(obj.Object, "conditions")
			assert(t, len(actual), test.expected)
		})
	}
}

func TestApplyKeepManifestivalAnnotation(t *testing.T) {
	ctx := context.Background()
	current := []byte(`
apiVersion: v1
kind: ComponentStatus
metadata:
  name: test
`)
	resource := &unstructured.Unstructured{}
	resource.SetAPIVersion("v1")
	resource.SetKind("ComponentStatus")
	resource.SetName("test")

	client := fake.New()
	manifest, _ := ManifestFrom(Reader(bytes.NewReader(current)), UseClient(client))

	manifest.Apply(ctx)

	obj, _ := client.Get(ctx, resource)
	if obj.GetAnnotations()["manifestival"] != "new" {
		t.Errorf("Resource did not contain 'manifestival' annotation after first apply")
	}

	manifest2, _ := ManifestFrom(Reader(bytes.NewReader(current)), UseClient(client))
	manifest2.Apply(ctx)

	obj, _ = client.Get(ctx, resource)
	if obj.GetAnnotations()["manifestival"] != "new" {
		t.Errorf("Resource did not contain 'manifestival' annotation after second apply")
	}
}

func TestAppend(t *testing.T) {
	ctx := context.Background()
	u := &unstructured.Unstructured{}
	u.SetName("testy")
	client := fake.New(u)
	m1, _ := NewManifest("testdata/tree/file.yaml", UseClient(client))
	m2, _ := NewManifest("testdata/tree/file.yaml")
	m3 := m1.Append(m2)
	m4 := m1.Append(m2, m3)
	assert(t, len(m1.Resources()), 2)
	assert(t, len(m2.Resources()), 2)
	assert(t, len(m3.Resources()), 4)
	assert(t, len(m4.Resources()), 8)

	assert(t, m2.Client == nil, true)
	for _, m := range []Manifest{m1, m3, m4} {
		q, _ := m.Client.Get(ctx, u)
		assert(t, q.GetName(), u.GetName())
	}
}

func TestNamespaceDeletion(t *testing.T) {
	ctx := context.Background()
	ns := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	u := unstructured.Unstructured{}
	scheme.Scheme.Convert(&ns, &u, nil)
	// First verify existing namespaces are *NOT* deleted
	client := fake.New(&ns)
	manifest, _ := ManifestFrom(Slice([]unstructured.Unstructured{u}), UseClient(client))
	err := manifest.Apply(ctx)
	assert(t, err, nil)
	err = manifest.Delete(ctx)
	assert(t, err, nil)
	_, err = client.Get(ctx, &u)
	assert(t, err, nil)
	// Now verify that newly-created namespaces *ARE* deleted
	err = client.Delete(ctx, &u)
	assert(t, err, nil)
	err = manifest.Apply(ctx)
	assert(t, err, nil)
	err = manifest.Delete(ctx)
	assert(t, err, nil)
	_, err = client.Get(ctx, &u)
	assert(t, errors.IsNotFound(err), true)
}

func TestFilterNamespaceDeletion(t *testing.T) {
	ctx := context.Background()
	ns := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	u := unstructured.Unstructured{}
	scheme.Scheme.Convert(&ns, &u, nil)
	// First verify existing namespaces are *NOT* deleted
	client := fake.New(&ns)
	manifest, _ := ManifestFrom(Slice([]unstructured.Unstructured{u}), UseClient(client))
	namespace := Any(ByKind("Namespace"))
	err := manifest.Filter(namespace).Apply(ctx)
	assert(t, err, nil)
	err = manifest.Delete(ctx)
	assert(t, err, nil)
	_, err = client.Get(ctx, &u)
	assert(t, err, nil)
	// Now verify that newly-created namespaces *ARE* deleted
	err = client.Delete(ctx, &u)
	assert(t, err, nil)
	err = manifest.Filter(namespace).Apply(ctx)
	assert(t, err, nil)
	err = manifest.Delete(ctx)
	assert(t, err, nil)
	_, err = client.Get(ctx, &u)
	assert(t, errors.IsNotFound(err), true)
}

func TestGenerateName(t *testing.T) {
	ctx := context.Background()
	job := unstructured.Unstructured{}
	job.SetAPIVersion("batch/v1")
	job.SetKind("Job")
	job.SetGenerateName("fubar-")
	count := 0
	client := fake.Client{
		Stubs: fake.Stubs{
			Get: func(ctx context.Context, u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				if u.GetName() == "" {
					return nil, errors.NewBadRequest("invalid")
				}
				return u, nil
			},
			Create: func(ctx context.Context, u *unstructured.Unstructured) error {
				count++
				return nil
			},
		},
	}
	manifest, _ := ManifestFrom(Slice([]unstructured.Unstructured{job}), UseClient(client))
	assert(t, len(manifest.Filter(ByName("")).Resources()), 1)
	expected := 5
	for i := 0; i < expected; i++ {
		manifest.Apply(ctx)
	}
	assert(t, count, expected)
}

func TestDeleteError(t *testing.T) {
	ctx := context.Background()
	client := fake.Client{
		Stubs: fake.Stubs{
			Get: func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				return nil, errors.NewBadRequest("TestDeleteError")
			},
		},
	}
	manifest, _ := NewManifest("testdata/k-s-v0.12.1.yaml", UseClient(client))
	err := manifest.Delete(ctx)
	assert(t, err.Error(), "TestDeleteError")
}

func TestContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := fake.New()
	manifest, _ := NewManifest("testdata/k-s-v0.12.1.yaml", UseClient(client))
	cancel()
	err := manifest.Apply(ctx)
	assert(t, err, context.Canceled)
}

func assert(t *testing.T, actual, expected interface{}) {
	t.Helper()
	if actual == expected {
		return
	}
	t.Fatalf("\nExpected: %v\n  Actual: %v", expected, actual)
}
