package manifestival

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/manifestival/manifestival/internal/overlay"
	"github.com/manifestival/manifestival/internal/patch"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Manifestival defines the operations allowed on a set of Kubernetes
// resources (typically, a set of YAML files, aka a manifest)
type Manifestival interface {
	// Either updates or creates all resources in the manifest
	Apply(ctx context.Context, opts ...ApplyOption) error
	// Deletes all resources in the manifest
	Delete(ctx context.Context, opts ...DeleteOption) error
	// Transforms the resources within a Manifest
	Transform(fns ...Transformer) (Manifest, error)
	// Filters resources in a Manifest; Predicates are AND'd
	Filter(fns ...Predicate) Manifest
	// Append the resources from other Manifests to create a new one
	Append(mfs ...Manifest) Manifest
	// Show how applying the manifest would change the cluster
	DryRun(ctx context.Context) ([]MergePatch, error)
}

// Manifest tracks a set of concrete resources which should be managed as a
// group using a Kubernetes client
type Manifest struct {
	resources                   []unstructured.Unstructured
	Client                      Client
	log                         logr.Logger
	lastAppliedConfigAnnotation string
}

var _ Manifestival = &Manifest{}

// Option follows the "functional object" idiom
type Option func(*Manifest)

// UseLogger will cause manifestival to log its actions
func UseLogger(log logr.Logger) Option {
	return func(m *Manifest) {
		m.log = log
	}
}

// UseClient enables interaction with the k8s API server
func UseClient(client Client) Option {
	return func(m *Manifest) {
		m.Client = client
	}
}

// UseLastAppliedConfigAnnotation sets an alternate name for the annotation used to track the last applied configuration (defaults to kubectl.kubernetes.io/last-applied-configuration)
// annotationName is a value specific to your application such as myapp.example.com/last-applied-configuration
func UseLastAppliedConfigAnnotation(annotationName string) Option {
	return func(m *Manifest) {
		m.lastAppliedConfigAnnotation = annotationName
	}
}

// NewManifest creates a Manifest from a comma-separated set of YAML
// files, directories, or URLs. It's equivalent to
// `ManifestFrom(Path(pathname))`
func NewManifest(pathname string, opts ...Option) (Manifest, error) {
	return ManifestFrom(Path(pathname), opts...)
}

// ManifestFrom creates a Manifest from any Source implementation
func ManifestFrom(src Source, opts ...Option) (m Manifest, err error) {
	m = Manifest{log: logr.Discard(), lastAppliedConfigAnnotation: v1.LastAppliedConfigAnnotation}
	for _, opt := range opts {
		opt(&m)
	}
	m.log.Info("Parsing manifest")
	m.resources, err = src.Parse()
	return
}

// Append creates a new Manifest by appending the resources from other
// Manifests onto this one. No equality checking is done, so for any
// resources sharing the same GVK+name, the last one will "win".
func (m Manifest) Append(mfs ...Manifest) Manifest {
	result := m
	result.resources = m.Resources() // deep copies
	for _, mf := range mfs {
		result.resources = append(result.resources, mf.Resources()...)
	}
	return result
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
func (m Manifest) Apply(ctx context.Context, opts ...ApplyOption) error {
	for _, spec := range m.resources {
		if err := m.apply(ctx, &spec, opts...); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes all resources in the Manifest
func (m Manifest) Delete(ctx context.Context, opts ...DeleteOption) error {
	a := make([]unstructured.Unstructured, len(m.resources))
	copy(a, m.resources) // shallow copy is fine
	// we want to delete in reverse order
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
	for _, spec := range a {
		if err := m.delete(ctx, &spec, opts...); err != nil {
			return err
		}
	}
	return nil
}

// apply updates or creates a particular resource
func (m Manifest) apply(ctx context.Context, spec *unstructured.Unstructured, opts ...ApplyOption) error {
	current, err := m.get(ctx, spec)
	if err != nil {
		return err
	}
	if current == nil {
		m.logResource("Creating", spec)
		current = spec.DeepCopy()
		annotate(current, "manifestival", resourceCreated)
		annotate(current, m.lastAppliedConfigAnnotation, lastApplied(current, m.lastAppliedConfigAnnotation))
		return m.Client.Create(ctx, current, opts...)
	} else {
		diff, err := patch.New(current, spec, m.lastAppliedConfigAnnotation)
		if err != nil {
			return err
		}
		if diff == nil {
			return nil
		}

		isResourceCreated := current.GetAnnotations()["manifestival"] == resourceCreated
		m.log.Info("Merging", "diff", diff)
		if err := diff.Merge(current); err != nil {
			return err
		}

		// Make sure the manifestival annotation is carried over.
		if isResourceCreated {
			annotate(current, "manifestival", resourceCreated)
		}

		return m.update(ctx, current, spec, opts...)
	}
}

// update a single resource
func (m Manifest) update(ctx context.Context, live, spec *unstructured.Unstructured, opts ...ApplyOption) error {
	m.logResource("Updating", live)
	annotate(live, m.lastAppliedConfigAnnotation, lastApplied(spec, m.lastAppliedConfigAnnotation))
	err := m.Client.Update(ctx, live, opts...)
	if errors.IsInvalid(err) && ApplyWith(opts).Overwrite {
		m.log.Error(err, "Failed to update merged resource, trying overwrite")
		overlay.Copy(spec.Object, live.Object)
		return m.Client.Update(ctx, live, opts...)
	}
	return err
}

// delete removes the specified object
func (m Manifest) delete(ctx context.Context, spec *unstructured.Unstructured, opts ...DeleteOption) error {
	current, err := m.get(ctx, spec)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	if !okToDelete(current) {
		return nil
	}
	m.logResource("Deleting", spec)
	return m.Client.Delete(ctx, spec, opts...)
}

// get collects a full resource body (or `nil`) from a partial
// resource supplied in `spec`
func (m Manifest) get(ctx context.Context, spec *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if spec.GetName() == "" && spec.GetGenerateName() != "" {
		// expected to be created; never fetched
		return nil, nil
	}
	result, err := m.Client.Get(ctx, spec)
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
func lastApplied(obj *unstructured.Unstructured, annotationName string) string {
	ann := obj.GetAnnotations()
	if len(ann) > 0 {
		delete(ann, annotationName)
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
