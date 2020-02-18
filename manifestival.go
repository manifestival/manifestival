package manifestival

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testing"
	"github.com/manifestival/manifestival/patch"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Manifestival allows group application of a set of Kubernetes resources
// (typically, a set of YAML files, aka a manifest) against a Kubernetes
// apiserver.
type Manifestival interface {
	// Either updates or creates all resources in the manifest
	ApplyAll(opts ...ApplyOption) error
	// Updates or creates a particular resource
	Apply(spec *unstructured.Unstructured, opts ...ApplyOption) error
	// Deletes all resources in the manifest
	DeleteAll(opts ...DeleteOption) error
	// Deletes a particular resource
	Delete(spec *unstructured.Unstructured, opts ...DeleteOption) error
	// Returns a copy of the resource from the api server, nil if not found
	Get(spec *unstructured.Unstructured) (*unstructured.Unstructured, error)
	// Transforms the resources within a Manifest
	Transform(fns ...Transformer) (*Manifest, error)
}

// Manifest tracks a set of concrete resources which should be managed as a
// group using a Kubernetes client provided by `NewManifest`.
type Manifest struct {
	Resources []unstructured.Unstructured
	client    Client
	log       logr.Logger
}

var _ Manifestival = &Manifest{}

// NewManifest creates a Manifest from a comma-separated set of yaml
// files, directories, or URLs. The Manifest's client and logger may
// be optionally provided.
func NewManifest(pathname string, opts ...Option) (Manifest, error) {
	return ManifestFrom(Path(pathname), opts...)
}

// ManifestFrom creates a Manifest from any Source
func ManifestFrom(src Source, opts ...Option) (m Manifest, err error) {
	m = Manifest{log: testing.NullLogger{}}
	for _, opt := range opts {
		opt(&m)
	}
	m.log.Info("Parsing manifest")
	m.Resources, err = src.Parse()
	return
}

// ApplyAll updates or creates all resources in the manifest.
func (f *Manifest) ApplyAll(opts ...ApplyOption) error {
	for _, spec := range f.Resources {
		if err := f.Apply(&spec, opts...); err != nil {
			return err
		}
	}
	return nil
}

// Apply updates or creates a particular resource, which does not need to be
// part of `Resources`, and will not be tracked.
func (f *Manifest) Apply(spec *unstructured.Unstructured, opts ...ApplyOption) error {
	current, err := f.Get(spec)
	if err != nil {
		return err
	}
	if current == nil {
		f.logResource("Creating", spec)
		annotate(spec, v1.LastAppliedConfigAnnotation, patch.MakeLastAppliedConfig(spec))
		annotate(spec, "manifestival", resourceCreated)
		if err = f.client.Create(spec.DeepCopy(), opts...); err != nil {
			return err
		}
	} else {
		patch, err := patch.NewPatch(spec, current)
		if err != nil {
			return err
		}
		if patch.IsRequired() {
			f.log.Info("Merging", "diff", patch)
			if err := patch.Merge(current); err != nil {
				return err
			}
			f.logResource("Updating", current)
			if err = f.client.Update(current, opts...); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteAll removes all tracked `Resources` in the Manifest.
func (f *Manifest) DeleteAll(opts ...DeleteOption) error {
	a := make([]unstructured.Unstructured, len(f.Resources))
	copy(a, f.Resources)
	// we want to delete in reverse order
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
	for _, spec := range a {
		if okToDelete(&spec) {
			if err := f.Delete(&spec, opts...); err != nil {
				f.log.Error(err, "Delete failed")
			}
		}
	}
	return nil
}

// Delete removes the specified objects, which do not need to be registered as
// `Resources` in the Manifest.
func (f *Manifest) Delete(spec *unstructured.Unstructured, opts ...DeleteOption) error {
	current, err := f.Get(spec)
	if current == nil && err == nil {
		return nil
	}
	f.logResource("Deleting", spec)
	return f.client.Delete(spec, opts...)
}

// Get collects a full resource body (or `nil`) from a partial resource
// supplied in `spec`.
func (f *Manifest) Get(spec *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	result, err := f.client.Get(spec)
	if err != nil {
		result = nil
		if errors.IsNotFound(err) {
			err = nil
		}
	}
	return result, err
}

func (f *Manifest) logResource(msg string, spec *unstructured.Unstructured) {
	name := fmt.Sprintf("%s/%s", spec.GetNamespace(), spec.GetName())
	f.log.Info(msg, "name", name, "type", spec.GroupVersionKind())
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
