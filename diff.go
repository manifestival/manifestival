package manifestival

import (
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/manifestival/manifestival/patch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
)

type JSONMergePatch map[string]interface{}

// Diff returns a list of JSON Merge Patches [RFC 7386] that represent
// the changes that will occur when the manifest is applied
func (m Manifest) Diff(strategic bool) ([]JSONMergePatch, error) {
	diffs, err := m.diff(strategic)
	if err != nil {
		return nil, err
	}
	result := make([]JSONMergePatch, len(diffs))
	for i, bytes := range diffs {
		if err := json.Unmarshal(bytes, &result[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// diff loads the resources in the manifest and computes their difference
func (m Manifest) diff(strategic bool) ([][]byte, error) {
	result := make([][]byte, 0, len(m.resources))
	for _, spec := range m.resources {
		original, err := m.Client.Get(&spec)
		if err != nil {
			if errors.IsNotFound(err) {
				// this resource will be created when applied
				jmp, _ := spec.MarshalJSON()
				result = append(result, jmp)
				continue
			}
			return nil, err
		}
		diff, err := patch.New(original, &spec)
		if err != nil {
			return nil, err
		}
		if diff == nil {
			// ignore things that won't change
			continue
		}
		modified := original.DeepCopy()
		if err := diff.Merge(modified); err != nil {
			return nil, err
		}
		// Remove these fields so they'll be included in the patch
		original.SetAPIVersion("")
		original.SetKind("")
		original.SetName("")
		jmp, err := mergePatch(original, modified, strategic)
		if err != nil {
			return nil, err
		}
		result = append(result, jmp)
	}
	return result, nil
}

// mergePatch returns a 2-way merge patch
func mergePatch(orig, mod *unstructured.Unstructured, strategic bool) (_ []byte, err error) {
	var original, modified []byte
	if original, err = orig.MarshalJSON(); err != nil {
		return
	}
	if modified, err = mod.MarshalJSON(); err != nil {
		return
	}
	obj, err := scheme.Scheme.New(mod.GroupVersionKind())
	switch {
	case !strategic || runtime.IsNotRegisteredError(err):
		return jsonpatch.CreateMergePatch(original, modified)
	case err != nil:
		return nil, err
	default:
		return strategicpatch.CreateTwoWayMergePatch(original, modified, obj)
	}
}
