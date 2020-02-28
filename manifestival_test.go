package manifestival_test

import (
	"bytes"
	"context"
	"testing"

	logr "github.com/go-logr/logr/testing"
	. "github.com/manifestival/manifestival"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLastAppliedAnnotation(t *testing.T) {
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
	client := testClient(&cm)
	// Use the unstructured for our manifest
	m, _ := ManifestFrom(Slice([]unstructured.Unstructured{u}), UseClient(client))
	if err := m.Apply(); err != nil {
		t.Error(err)
	}
	x, _ := m.Client.Get(&u)
	actual := x.GetAnnotations()[v1.LastAppliedConfigAnnotation]
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestMethodChaining(t *testing.T) {
	const expected = 6
	const kind = "Deployment"
	const name = "controller"
	manifest, _ := NewManifest("testdata/k-s-v0.12.1.yaml", UseClient(testClient()), UseLogger(logr.TestLogger{T: t}))
	// Filter->Transform->Resources
	deployments, _ := manifest.Filter(ByKind(kind)).Transform(InjectNamespace("foo"))
	if len(deployments.Resources()) != expected {
		t.Errorf("Expected %d deployments, got %d", expected, len(deployments.Resources()))
	}
	// Filter->Apply
	if err := manifest.Filter(ByKind(kind)).Apply(); err != nil {
		t.Error(err, "Expected deployments to be applied")
	}
	// Filter->Resources
	u := manifest.Filter(ByKind(kind), ByName(name)).Resources()[0]
	if _, err := manifest.Client.Get(&u); err != nil {
		t.Error(err, "Expected controller deployment to be created")
	}
	// Filter->Delete
	if err := manifest.Filter(ByKind(kind), ByName(name)).Delete(); err != nil {
		t.Error(err)
	}
	if _, err := manifest.Client.Get(&u); !errors.IsNotFound(err) {
		t.Error(err, "Expected controller deployment to be deleted")
	}
}

func TestReplaceApply(t *testing.T) {
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
		name     string
		replace  bool
		expected int
	}{{
		name:     "Merge patch",
		replace:  false,
		expected: 2,
	}, {
		name:     "Replace",
		replace:  true,
		expected: 1,
	}}
	setup, _ := ManifestFrom(Reader(current))
	original := setup.Resources()[0]
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := testClient(&original)
			tgt, _ := ManifestFrom(Reader(config), UseClient(client))
			tgt.Apply(Replace(test.replace))
			obj, _ := tgt.Client.Get(&original)
			actual, _, _ := unstructured.NestedSlice(obj.Object, "conditions")
			if len(actual) != test.expected {
				t.Errorf("Nope! Expected %v, got %v", test.expected, len(actual))
			}
		})
	}
}

func testClient(objs ...runtime.Object) Client {
	return &fakeClient{client: fake.NewFakeClient(objs...)}
}

type fakeClient struct {
	client client.Client
}

var _ Client = (*fakeClient)(nil)

func (c *fakeClient) Create(obj *unstructured.Unstructured, options ...ApplyOption) error {
	return c.client.Create(context.TODO(), obj)
}

func (c *fakeClient) Update(obj *unstructured.Unstructured, options ...ApplyOption) error {
	return c.client.Update(context.TODO(), obj)
}

func (c *fakeClient) Delete(obj *unstructured.Unstructured, options ...DeleteOption) error {
	return client.IgnoreNotFound(c.client.Delete(context.TODO(), obj))
}

func (c *fakeClient) Get(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	key := client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}
	result := &unstructured.Unstructured{}
	result.SetGroupVersionKind(obj.GroupVersionKind())
	err := c.client.Get(context.TODO(), key, result)
	return result, err
}
