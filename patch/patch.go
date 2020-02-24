package patch

import (
	"bytes"

	jsonpatch "github.com/evanphx/json-patch"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
)

type Patch interface {
	Empty() bool
	merge([]byte) ([]byte, error)
	lastApplied() string
}

type jsonMergePatch struct {
	patch  []byte
	config string
}

type strategicMergePatch struct {
	jsonMergePatch
	schema strategicpatch.LookupPatchMeta
}

// Attempts to create a 3-way strategic merge patch. Falls back to
// RFC-7386 if object's type isn't registered or rfc7386 is true
func New(src, tgt *unstructured.Unstructured, rfc7386 bool) (result Patch, err error) {
	var original, modified, current []byte
	original = getLastAppliedConfig(tgt)
	config := MakeLastAppliedConfig(src)
	if modified, err = src.MarshalJSON(); err != nil {
		return
	}
	if current, err = tgt.MarshalJSON(); err != nil {
		return
	}
	obj, err := scheme.Scheme.New(src.GroupVersionKind())
	switch {
	case rfc7386:
		fallthrough // force "replace" merge for list types
	case runtime.IsNotRegisteredError(err):
		return createJsonMergePatch(original, modified, current, config)
	case err != nil:
		return
	default:
		return createStrategicMergePatch(original, modified, current, obj, config)
	}
}

// Apply the patch to the resource
func Apply(p Patch, obj *unstructured.Unstructured) (err error) {
	var current, result []byte
	if current, err = obj.MarshalJSON(); err != nil {
		return
	}
	if result, err = p.merge(current); err != nil {
		return
	}
	err = obj.UnmarshalJSON(result)
	if err == nil {
		setLastAppliedConfig(obj, p.lastApplied())
	}
	return
}

// MakeLastAppliedConfig for the resource
func MakeLastAppliedConfig(obj *unstructured.Unstructured) string {
	ann := obj.GetAnnotations()
	if len(ann) > 0 {
		delete(ann, v1.LastAppliedConfigAnnotation)
		obj.SetAnnotations(ann)
	}
	bytes, _ := obj.MarshalJSON()
	return string(bytes)
}

func createJsonMergePatch(original, modified, current []byte, config string) (Patch, error) {
	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, current)
	return &jsonMergePatch{patch, config}, err
}

func createStrategicMergePatch(original, modified, current []byte, obj runtime.Object, config string) (Patch, error) {
	schema, err := strategicpatch.NewPatchMetaFromStruct(obj)
	if err != nil {
		return nil, err
	}
	patch, err := strategicpatch.CreateThreeWayMergePatch(original, modified, current, schema, true)
	return &strategicMergePatch{jsonMergePatch{patch, config}, schema}, err
}

var emptyDiff = []byte("{}")

func (p *jsonMergePatch) Empty() bool {
	return bytes.Equal(p.patch, emptyDiff)
}
func (p *jsonMergePatch) String() string {
	return string(p.patch)
}
func (p *jsonMergePatch) lastApplied() string {
	return p.config
}
func (p *jsonMergePatch) merge(obj []byte) ([]byte, error) {
	return jsonpatch.MergePatch(obj, p.patch)
}

func (p *strategicMergePatch) merge(obj []byte) ([]byte, error) {
	return strategicpatch.StrategicMergePatchUsingLookupPatchMeta(obj, p.patch, p.schema)
}

func getLastAppliedConfig(obj *unstructured.Unstructured) []byte {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}
	return []byte(annotations[v1.LastAppliedConfigAnnotation])
}

func setLastAppliedConfig(obj *unstructured.Unstructured, config string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[v1.LastAppliedConfigAnnotation] = config
	obj.SetAnnotations(annotations)
}
