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

// Manifestival defines the operations allowed on a set of Kubernetes
// resources (typically, a set of YAML files, aka a manifest)
type Manifestival interface {
	// Either updates or creates all resources in the manifest
	Apply(opts ...ApplyOption) error
	// Deletes all resources in the manifest
	Delete(opts ...DeleteOption) error
	// Transforms the resources within a Manifest
	Transform(fns ...Transformer) (Manifest, error)
	// Filters resources in a Manifest; Predicates are AND'd
	Filter(fns ...Predicate) Manifest
	// Show how applying the manifest would change the cluster
	DryRun() ([]MergePatch, error)
}

// Manifest tracks a set of concrete resources which should be managed as a
// group using a Kubernetes client
type Manifest struct {
	resources []unstructured.Unstructured
	Client    Client
	log       logr.Logger
}

var _ Manifestival = &Manifest{}

// NewManifest creates a Manifest from a comma-separated set of yaml
// files, directories, or URLs
func NewManifest(pathname string, opts ...Option) (Manifest, error) {
	return ManifestFrom(Path(pathname), opts...)
}

// ManifestFrom creates a Manifest from any Source implementation
func ManifestFrom(src Source, opts ...Option) (m Manifest, err error) {
	m = Manifest{log: testing.NullLogger{}}
	for _, opt := range opts {
		opt(&m)
	}
	m.log.Info("Parsing manifest")
	m.resources, err = src.Parse()
	return
}

// Resources returns a deep copy of the Manifest resources
func (m Manifest) Resources() []unstructured.Unstructured {
	result := make([]unstructured.Unstructured, len(m.resources))
	for i, v := range m.resources {
		result[i] = *v.DeepCopy()
	}
	return result
}

// Apply updates or creates all resources in the manifest.
func (m Manifest) Apply(opts ...ApplyOption) error {
	for _, spec := range m.resources {
		if err := m.apply(&spec, opts...); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes all resources in the Manifest
func (m Manifest) Delete(opts ...DeleteOption) error {
	a := make([]unstructured.Unstructured, len(m.resources))
	copy(a, m.resources) // shallow copy is fine
	// we want to delete in reverse order
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
	for _, spec := range a {
		if okToDelete(&spec) {
			if err := m.delete(&spec, opts...); err != nil {
				return err
			}
		}
	}
	return nil
}

// apply updates or creates a particular resource
func (m Manifest) apply(spec *unstructured.Unstructured, opts ...ApplyOption) error {
	current, err := m.get(spec)
	if err != nil {
		return err
	}
	if current == nil {
		m.logResource("Creating", spec)
		current = spec.DeepCopy()
		annotate(current, v1.LastAppliedConfigAnnotation, lastApplied(current))
		annotate(current, "manifestival", resourceCreated)
		return m.Client.Create(current, opts...)
	} else {
		if ApplyWith(opts).Replace {
			return m.update(spec.DeepCopy(), lastApplied(spec), opts...)
		}
		diff, err := patch.New(current, spec)
		if err != nil {
			return err
		}
		if diff != nil {
			m.log.Info("Merging", "diff", diff)
			if err := diff.Merge(current); err != nil {
				return err
			}
			return m.update(current, lastApplied(spec), opts...)
		}
	}
	return nil
}

// update a single resource
func (m Manifest) update(obj *unstructured.Unstructured, config string, opts ...ApplyOption) error {
	m.logResource("Updating", obj)
	annotate(obj, v1.LastAppliedConfigAnnotation, config)
	return m.Client.Update(obj, opts...)
}

// delete removes the specified object
func (m Manifest) delete(spec *unstructured.Unstructured, opts ...DeleteOption) error {
	current, err := m.get(spec)
	if current == nil && err == nil {
		return nil
	}
	m.logResource("Deleting", spec)
	return m.Client.Delete(spec, opts...)
}

// get collects a full resource body (or `nil`) from a partial
// resource supplied in `spec`
func (m Manifest) get(spec *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	result, err := m.Client.Get(spec)
	if err != nil {
		result = nil
		if errors.IsNotFound(err) {
			err = nil
		}
	}
	return result, err
}

// logResource logs a consistent formatted message
func (m Manifest) logResource(msg string, spec *unstructured.Unstructured) {
	name := fmt.Sprintf("%s/%s", spec.GetNamespace(), spec.GetName())
	m.log.Info(msg, "name", name, "type", spec.GroupVersionKind())
}

// annotate sets an annotation in the resource
func annotate(spec *unstructured.Unstructured, key string, value string) {
	annotations := spec.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	spec.SetAnnotations(annotations)
}

// lastApplied returns a JSON string denoting the resource's state
func lastApplied(obj *unstructured.Unstructured) string {
	ann := obj.GetAnnotations()
	if len(ann) > 0 {
		delete(ann, v1.LastAppliedConfigAnnotation)
		obj.SetAnnotations(ann)
	}
	bytes, _ := obj.MarshalJSON()
	return string(bytes)
}

// okToDelete checks for an annotation indicating that the resources
// was originally created by this library
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
