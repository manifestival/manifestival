package manifestival_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	logr "github.com/go-logr/logr/testing"
	. "github.com/manifestival/manifestival"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManifestChaining(t *testing.T) {
	const expected = 6
	const kind = "Deployment"
	const name = "controller"
	manifest, _ := NewManifest("testdata/knative-serving.yaml", UseClient(testClient()), UseLogger(logr.TestLogger{T: t}))
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

func TestApply(t *testing.T) {
	input := bytes.NewReader([]byte(`
apiVersion: v1
kind: ComponentStatus
metadata:
  name: test
conditions:
- type: foo
  status: "True"
`))
	tests := []struct {
		name     string
		replace  bool
		input    io.Reader
		expected map[string]string
	}{{
		name:    "Merge patch",
		replace: false,
		input: bytes.NewReader([]byte(`
apiVersion: v1
kind: ComponentStatus
metadata:
  name: test
conditions:
- type: bar
  status: "False"
`)),
		expected: map[string]string{"foo": "True", "bar": "False"},
	}, {
		name:    "Replace",
		replace: true,
		input: bytes.NewReader([]byte(`
apiVersion: v1
kind: ComponentStatus
metadata:
  name: test
conditions:
- type: bar
  status: "False"
`)),
		expected: map[string]string{"bar": "False"},
	}}
	setup, _ := ManifestFrom(Reader(input))
	original := setup.Resources()[0]
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := testClient(&original)
			tgt, _ := ManifestFrom(Reader(test.input), UseClient(client))
			tgt.Apply(Replace(test.replace))
			obj, _ := tgt.Client.Get(&original)
			actual, _, _ := unstructured.NestedSlice(obj.Object, "conditions")
			if len(actual) != len(test.expected) {
				t.Errorf("Nope! Expected %v, got %v", test.expected, actual)
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
