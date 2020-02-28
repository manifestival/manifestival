package manifestival

import (
	"encoding/json"

	"github.com/manifestival/manifestival/patch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type JSONMergePatch map[string]interface{}

// Diff returns a list of JSON Merge Patches [RFC 7386] that represent
// the changes that will occur when the manifest is applied
func (m Manifest) Diff() ([]JSONMergePatch, error) {
	diffs, err := m.diff()
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
func (m Manifest) diff() (result [][]byte, err error) {
	var original, modified *unstructured.Unstructured
	var jmp []byte
	var diff patch.Patch
	for _, spec := range m.resources {
		if original, err = m.Client.Get(&spec); err != nil {
			if errors.IsNotFound(err) {
				// this resource will be created when applied
				jmp, _ = patch.TwoWay(nil, &spec, false)
				result = append(result, jmp)
				continue
			}
			return
		}
		modified = original.DeepCopy()
		if diff, err = patch.New(&spec, modified); err != nil {
			return
		}
		if diff == nil {
			// ignore things that won't change
			continue
		}
		if err = diff.Apply(modified); err != nil {
			return
		}
		// Remove these fields so they'll be included in the patch
		original.SetAPIVersion("")
		original.SetKind("")
		original.SetName("")
		if jmp, err = patch.TwoWay(original, modified, false); err != nil {
			return
		}
		result = append(result, jmp)
	}
	return
}
