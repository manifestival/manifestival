package manifestival_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	logr "github.com/go-logr/logr/testing"
	. "github.com/manifestival/manifestival"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/pkg/apis/apps"
	corev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/registry/apps/deployment"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	corev1.AddToScheme(scheme.Scheme)
	scheme.Scheme.AddConversionFuncs(
		func(in *appsv1.RollingUpdateDeployment, out *apps.RollingUpdateDeployment, scope conversion.Scope) error {
			out.MaxUnavailable = *in.MaxUnavailable
			out.MaxSurge = *in.MaxSurge
			return nil
		},
	)
}

func TestPortUpdates(t *testing.T) {
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
	c := testClient(edited)
	manifest, _ := ManifestFrom(Slice([]unstructured.Unstructured{*spec}), UseClient(c), UseLogger(logr.TestLogger{T: t}))
	if err := manifest.Apply(Overwrite(false)); err == nil {
		t.Error("Should have received an invalid error")
	}
	if err := manifest.Apply(); err != nil {
		t.Error(err)
	}
}

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
			client := testClient(&original)
			tgt, _ := ManifestFrom(Reader(config), UseClient(client))
			tgt.Apply(Overwrite(test.overwrite))
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
var deploymentor = deployment.Strategy

func (c *fakeClient) Create(obj *unstructured.Unstructured, options ...ApplyOption) error {
	return c.client.Create(context.TODO(), obj)
}

func (c *fakeClient) Update(obj *unstructured.Unstructured, options ...ApplyOption) error {
	if obj.GetKind() == "Deployment" {
		dObj := &apps.Deployment{}
		scheme.Scheme.Convert(obj, dObj, nil)
		if errs := deploymentor.Validate(context.TODO(), dObj); len(errs) > 0 {
			return errors.NewInvalid(obj.GroupVersionKind().GroupKind(), obj.GetName(), errs)
		}
	}
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
