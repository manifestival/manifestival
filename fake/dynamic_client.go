package fake

import (
	mf "github.com/manifestival/manifestival"

	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
)

type fakeDynamicClient struct {
	kfdc *fake.FakeDynamicClient
}

// NewFakeDynamicClient returns a Manifestival client backed by a kubernets FakeDynamicClient
func NewFakeDynamicClient(kfdc *fake.FakeDynamicClient) fakeDynamicClient {
	return fakeDynamicClient{
		kfdc: kfdc,
	}
}

func (fdc fakeDynamicClient) getResource(obj *unstructured.Unstructured) dynamic.ResourceInterface {
	plural, _ := meta.UnsafeGuessKindToResource(obj.GroupVersionKind())
	return fdc.kfdc.Resource(plural).Namespace(obj.GetNamespace())
}

func (fdc fakeDynamicClient) Create(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	opts := mf.ApplyWith(options)
	_, err := fdc.getResource(obj).Create(obj, *opts.ForCreate)
	return err
}

func (fdc fakeDynamicClient) Update(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	opts := mf.ApplyWith(options)
	_, err := fdc.getResource(obj).Update(obj, *opts.ForUpdate)
	return err
}

func (fdc fakeDynamicClient) Delete(obj *unstructured.Unstructured, options ...mf.DeleteOption) error {
	return fdc.getResource(obj).Delete(obj.GetName(), mf.DeleteWith(options).ForDelete)
}

func (fdc fakeDynamicClient) Get(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return fdc.getResource(obj).Get(obj.GetName(), v1.GetOptions{})
}

var _ mf.Client = (*fakeDynamicClient)(nil)
