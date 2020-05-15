package fake

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ mf.Client = &Client{}

type mutator func(obj *unstructured.Unstructured) error
type accessor func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error)

type Stubs struct {
	Create mutator
	Update mutator
	Delete mutator
	Get    accessor
}

type Client struct {
	Stubs
}

func (c Client) Create(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	if c.Stubs.Create != nil {
		return c.Stubs.Create(obj)
	}
	return nil
}

func (c Client) Update(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	if c.Stubs.Update != nil {
		return c.Stubs.Update(obj)
	}
	return nil
}

func (c Client) Delete(obj *unstructured.Unstructured, options ...mf.DeleteOption) error {
	if c.Stubs.Delete != nil {
		return c.Stubs.Delete(obj)
	}
	return nil
}

func (c Client) Get(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if c.Stubs.Get != nil {
		return c.Stubs.Get(obj)
	}
	return nil, nil
}

func New(objs ...runtime.Object) Client {
	store := map[string]*unstructured.Unstructured{}
	key := func(u *unstructured.Unstructured) string {
		return fmt.Sprintf("%s, %s/%s", u.GroupVersionKind(), u.GetNamespace(), u.GetName())
	}
	for _, obj := range objs {
		u := &unstructured.Unstructured{}
		if err := scheme.Scheme.Convert(obj, u, nil); err != nil {
			panic(err)
		}
		store[key(u)] = u
	}
	apply := func(u *unstructured.Unstructured) error {
		store[key(u)] = u
		return nil
	}
	return Client{
		Stubs{
			Create: apply,
			Update: apply,
			Delete: func(u *unstructured.Unstructured) error {
				delete(store, key(u))
				return nil
			},
			Get: func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				v, found := store[key(u)]
				if !found {
					gvk := u.GroupVersionKind()
					gr := schema.GroupResource{gvk.Group, gvk.Kind}
					return nil, errors.NewNotFound(gr, u.GetName())
				}
				return v, nil
			},
		},
	}
}
