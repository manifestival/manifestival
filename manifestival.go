package manifestival

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/manifestival/manifestival/patch"
	"github.com/operator-framework/operator-sdk/pkg/restmapper"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var log = zapr.NewLogger(zap.NewExample())

func SetLogger(l logr.Logger) {
	log = l.WithName("manifestival")
}

// Manifestival allows group application of a set of Kubernetes resources
// (typically, a set of YAML files, aka a manifest) against a Kubernetes
// apiserver.
type Manifestival interface {
	// Either updates or creates all resources in the manifest
	ApplyAll() error
	// Updates or creates a particular resource
	Apply(*unstructured.Unstructured) error
	// Deletes all resources in the manifest
	DeleteAll(opts *metav1.DeleteOptions) error
	// Deletes a particular resource
	Delete(spec *unstructured.Unstructured, opts *metav1.DeleteOptions) error
	// Returns a copy of the resource from the api server, nil if not found
	Get(spec *unstructured.Unstructured) (*unstructured.Unstructured, error)
	// Transforms the resources within a Manifest
	Transform(fns ...Transformer) (*Manifest, error)
}

// Manifest tracks a set of concrete resources which should be managed as a
// group using a Kubernetes client provided by `NewManifest`.
type Manifest struct {
	Resources []unstructured.Unstructured
	client    dynamic.Interface
	mapper    meta.RESTMapper
}

var _ Manifestival = &Manifest{}

// NewManifest creates a Manifest from a comma-separated set of yaml files or
// directories (and subdirectories if the `recursive` option is set). The
// Manifest will be evaluated using the supplied `config` against a particular
// Kubernetes apiserver.
func NewManifest(pathname string, recursive bool, config *rest.Config) (Manifest, error) {
	log.Info("Reading manifest", "name", pathname)
	resources, err := Parse(pathname, recursive)
	if err != nil {
		return Manifest{}, err
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return Manifest{Resources: resources}, err
	}
	mapper, err := restmapper.NewDynamicRESTMapper(config)
	if err != nil {
		return Manifest{Resources: resources}, err
	}
	return Manifest{Resources: resources, client: client, mapper: mapper}, nil
}

// ApplyAll updates or creates all resources in the manifest.
func (f *Manifest) ApplyAll() error {
	for _, spec := range f.Resources {
		if err := f.Apply(&spec); err != nil {
			return err
		}
	}
	return nil
}

// Apply updates or creates a particular resource, which does not need to be
// part of `Resources`, and will not be tracked.
func (f *Manifest) Apply(spec *unstructured.Unstructured) error {
	current, err := f.Get(spec)
	if err != nil {
		return err
	}
	resource, err := f.ResourceInterface(spec)
	if err != nil {
		return err
	}
	if current == nil {
		logResource("Creating", spec)
		annotate(spec, v1.LastAppliedConfigAnnotation, patch.MakeLastAppliedConfig(spec))
		annotate(spec, "manifestival", resourceCreated)
		if _, err = resource.Create(spec, metav1.CreateOptions{}); err != nil {
			return err
		}
	} else {
		patch, err := patch.NewPatch(spec, current)
		if err != nil {
			return err
		}
		if patch.IsRequired() {
			log.Info("Merging", "diff", patch)
			if err := patch.Merge(current); err != nil {
				return err
			}
			logResource("Updating", current)
			if _, err = resource.Update(current, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteAll removes all tracked `Resources` in the Manifest.
func (f *Manifest) DeleteAll(opts *metav1.DeleteOptions) error {
	a := make([]unstructured.Unstructured, len(f.Resources))
	copy(a, f.Resources)
	// we want to delete in reverse order
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
	for _, spec := range a {
		if okToDelete(&spec) {
			if err := f.Delete(&spec, opts); err != nil {
				log.Error(err, "Delete failed")
			}
		}
	}
	return nil
}

// Delete removes the specified objects, which do not need to be registered as
// `Resources` in the Manifest.
func (f *Manifest) Delete(spec *unstructured.Unstructured, opts *metav1.DeleteOptions) error {
	current, err := f.Get(spec)
	if current == nil && err == nil {
		return nil
	}
	logResource("Deleting", spec)
	resource, err := f.ResourceInterface(spec)
	if err != nil {
		return err
	}
	if err := resource.Delete(spec.GetName(), opts); err != nil {
		// ignore GC race conditions triggered by owner references
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// Get collects a full resource body (or `nil`) from a partial resource
// supplied in `spec`.
func (f *Manifest) Get(spec *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	resource, err := f.ResourceInterface(spec)
	if err != nil {
		return nil, err
	}
	result, err := resource.Get(spec.GetName(), metav1.GetOptions{})
	if err != nil {
		result = nil
		if errors.IsNotFound(err) {
			err = nil
		}
	}
	return result, err
}

// ResourceInterface returns an interface appropriate for the spec
func (f *Manifest) ResourceInterface(spec *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	gvk := spec.GroupVersionKind()
	mapping, err := f.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	if mapping.Scope.Name() == meta.RESTScopeNameRoot {
		return f.client.Resource(mapping.Resource), nil
	}
	return f.client.Resource(mapping.Resource).Namespace(spec.GetNamespace()), nil
}

func logResource(msg string, spec *unstructured.Unstructured) {
	name := fmt.Sprintf("%s/%s", spec.GetNamespace(), spec.GetName())
	log.Info(msg, "name", name, "type", spec.GroupVersionKind())
}

func annotate(spec *unstructured.Unstructured, key string, value string) {
	annotations := spec.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	spec.SetAnnotations(annotations)
}

func okToDelete(spec *unstructured.Unstructured) bool {
	switch spec.GetKind() {
	case "Namespace":
		return spec.GetAnnotations()["manifestival"] == resourceCreated
	}
	return true
}

const (
	resourceCreated = "new"
)
