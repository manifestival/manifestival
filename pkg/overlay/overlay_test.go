package overlay_test

import (
	"reflect"
	"testing"

	"github.com/manifestival/manifestival/overlay"
	"sigs.k8s.io/yaml"
)

type overlayTestCase struct {
	Name   string
	Source map[string]interface{}
	Target map[string]interface{}
	Expect map[string]interface{}
}

var testdata = []byte(`
- name: identical maps
  source:
    x: foo
  target:
    x: foo
  expect:
    x: foo
- name: changed key value
  source:
    x:
      name: larry
  target:
    x:
      name: curly
  expect:
    x:
      name: larry
- name: extra map key
  source:
    x:
      name: moe
  target:
    x:
      y: 2
  expect:
    x:
      y: 2
      name: moe
- name: retain extra key in target list
  source:
    x:
    - name: larry
      age: 23
  target:
    x:
    - name: curly
      age: 42
      clusterIP: 1.2.3.4
  expect:
    x:
    - name: larry
      age: 23
      clusterIP: 1.2.3.4
- name: too many in target
  source:
    x:
    - name: larry
      age: 23
  target:
    x:
    - name: curly
      age: 42
    - name: larry
      age: 66
  expect:
    x:
    - name: larry
      age: 23
- name: too few in target
  source:
    x:
    - name: curly
      age: 42
    - name: larry
      age: 66
  target:
    x:
    - t: texas
      y: ynot
  expect:
    x:
    - name: curly
      age: 42
    - name: larry
      age: 66
- name: different types
  source:
    x:
      name: curly
      age: 42
  target:
    x:
    - name: curly
      age: 42
    - name: larry
      age: 66
  expect:
    x:
      name: curly
      age: 42
`)

func TestOverlay(t *testing.T) {
	tests := []overlayTestCase{}
	err := yaml.Unmarshal(testdata, &tests)
	if err != nil {
		t.Error(err)
		return
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			overlay.Copy(test.Source, test.Target)
			if !reflect.DeepEqual(test.Target, test.Expect) {
				t.Errorf("\n     got %s\nexpected %s", test.Target, test.Expect)
			}
		})
	}
}
