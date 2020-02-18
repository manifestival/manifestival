package manifestival

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Client interface {
	Create(obj *unstructured.Unstructured, options ...ApplyOption) error
	Update(obj *unstructured.Unstructured, options ...ApplyOption) error
	Delete(obj *unstructured.Unstructured, options ...DeleteOption) error
	Get(obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
}

func ApplyWith(options []ApplyOption) *ApplyOptions {
	result := &ApplyOptions{
		ForCreate: &metav1.CreateOptions{},
		ForUpdate: &metav1.UpdateOptions{},
	}
	for _, f := range options {
		f.ApplyWith(result)
	}
	return result
}

func DeleteWith(options []DeleteOption) *DeleteOptions {
	result := &DeleteOptions{
		ForDelete:      &metav1.DeleteOptions{},
		IgnoreNotFound: true,
	}
	for _, f := range options {
		f.DeleteWith(result)
	}
	return result
}

// Functional options pattern
type ApplyOption interface {
	ApplyWith(*ApplyOptions)
}
type DeleteOption interface {
	DeleteWith(*DeleteOptions)
}

type ApplyOptions struct {
	ForCreate *metav1.CreateOptions
	ForUpdate *metav1.UpdateOptions
}
type DeleteOptions struct {
	ForDelete      *metav1.DeleteOptions
	IgnoreNotFound bool // default to true in OptionsForDelete()
}

var DryRunAll = dryRunAll{}

type FieldManager string
type GracePeriodSeconds int64
type Preconditions metav1.Preconditions
type PropagationPolicy metav1.DeletionPropagation
type IgnoreNotFound bool
type dryRunAll struct{} // for both apply and delete

func (dryRunAll) ApplyWith(opts *ApplyOptions) {
	opts.ForCreate.DryRun = []string{metav1.DryRunAll}
	opts.ForUpdate.DryRun = []string{metav1.DryRunAll}
}
func (f FieldManager) ApplyWith(opts *ApplyOptions) {
	fm := string(f)
	opts.ForCreate.FieldManager = fm
	opts.ForUpdate.FieldManager = fm
}

func (dryRunAll) DeleteWith(opts *DeleteOptions) {
	opts.ForDelete.DryRun = []string{metav1.DryRunAll}
}
func (g GracePeriodSeconds) DeleteWith(opts *DeleteOptions) {
	s := int64(g)
	opts.ForDelete.GracePeriodSeconds = &s
}
func (p Preconditions) DeleteWith(opts *DeleteOptions) {
	preconds := metav1.Preconditions(p)
	opts.ForDelete.Preconditions = &preconds
}
func (p PropagationPolicy) DeleteWith(opts *DeleteOptions) {
	policy := metav1.DeletionPropagation(p)
	opts.ForDelete.PropagationPolicy = &policy
}
func (i IgnoreNotFound) DeleteWith(opts *DeleteOptions) {
	opts.IgnoreNotFound = bool(i)
}