package patch_test

import (
	"bytes"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/manifestival/manifestival/patch"
)

type updateChangedTestCases struct {
	TestCases []updateChangedTestCase
}

type updateChangedTestCase struct {
	Name    string
	Changed bool
	Source  map[string]interface{}
	Target  map[string]interface{}
	Expect  map[string]interface{}
}

var testdata = []byte(`
testCases:
  - name: identical maps
    changed: false
    source:
      kind: T
      x:
        z: i
    target:
      kind: T
      x:
        z: i
    expect:
      kind: T
      x:
        z: i
  - name: add nested map entry
    changed: true
    source:
      kind: T
      x:
        z: i
    target:
      kind: T
      x:
        a: foo
    expect:
      kind: T
      metadata:
        annotations:
          kubectl.kubernetes.io/last-applied-configuration: |
            {"kind":"T","x":{"z":"i"}}
      x:
        z: i
        a: foo
  - name: change nested map entry
    changed: true
    source:
      kind: T
      x:
        z: i
    target:
      kind: T
      x:
        z: j
    expect:
      kind: T
      metadata:
        annotations:
          kubectl.kubernetes.io/last-applied-configuration: |
            {"kind":"T","x":{"z":"i"}}
      x:
        z: i
  - name: change missing map entry
    changed: true
    source:
      kind: T
      x:
        z: i
    target:
      kind: T
    expect:
      kind: T
      metadata:
        annotations:
          kubectl.kubernetes.io/last-applied-configuration: |
            {"kind":"T","x":{"z":"i"}}
      x:
        z: i
  - name: identical nested slice
    changed: false
    source:
      kind: T
      x:
        z:
          - i
          - j
    target:
      kind: T
      x:
        z:
          - i
          - j
    expect:
      kind: T
      x:
        z:
          - i
          - j
  - name: add nested slice entry
    changed: true
    source:
      kind: T
      x:
        z:
          - i
          - j
    target:
      kind: T
      x:
        z:
          - i
    expect:
      kind: T
      metadata:
        annotations:
          kubectl.kubernetes.io/last-applied-configuration: |
            {"kind":"T","x":{"z":["i","j"]}}
      x:
        z:
          - i
          - j
  - name: update nested slice entry
    changed: true
    source:
      kind: T
      x:
        z:
          - i
          - j
          - k
    target:
      kind: T
      x:
        z:
          - m
          - n
          - p
    expect:
      kind: T
      metadata:
        annotations:
          kubectl.kubernetes.io/last-applied-configuration: |
            {"kind":"T","x":{"z":["i","j","k"]}}
      x:
        z:
          - i
          - j
          - k
  - name: add missing slice entry
    changed: true
    source:
      kind: T
      x:
        z:
          - i
          - j
    target:
      kind: T
      x:
        x:
          - j
    expect:
      kind: T
      metadata:
        annotations:
          kubectl.kubernetes.io/last-applied-configuration: |
            {"kind":"T","x":{"z":["i","j"]}}
      x:
        z:
          - i
          - j
        x:
          - j
  - name: change map within list
    changed: true
    source:
      kind: T
      x:
        z:
          - foo: bar
    target:
      kind: T
      x:
        z:
          - foo: bar
            one: "1"
    expect:
      kind: T
      metadata:
        annotations:
          kubectl.kubernetes.io/last-applied-configuration: |
            {"kind":"T","x":{"z":[{"foo":"bar"}]}}
      x:
        z:
          - foo: bar
  - name: strategic patch # https://kubernetes.io/docs/tasks/manage-kubernetes-objects/declarative-config/#merge-patch-calculation
    changed: true
    source:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx-deployment
      spec:
        selector:
          matchLabels:
            app: nginx
        template:
          metadata:
            labels:
              app: nginx
          spec:
            containers:
            - name: nginx
              image: nginx:1.11.9 # update the image
              ports:
              - containerPort: 80
    target:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        annotations:
          # note that the annotation does not contain replicas
          # because it was not updated through apply
          kubectl.kubernetes.io/last-applied-configuration: |
            {"apiVersion":"apps/v1","kind":"Deployment","metadata":{"annotations":{},"name":"nginx-deployment","namespace":"default"},"spec":{"minReadySeconds":5,"selector":{"matchLabels":{"app":"nginx"}},"template":{"metadata":{"labels":{"app":"nginx"}},"spec":{"containers":[{"image":"nginx:1.7.9","name":"nginx","ports":[{"containerPort":80}]}]}}}}
      spec:
        replicas: 2 # written by scale
        minReadySeconds: 5
        selector:
          matchLabels:
            app: nginx
        template:
          metadata:
            labels:
              app: nginx
          spec:
            containers:
            - image: nginx:1.7.9
              name: nginx
              ports:
              - containerPort: 80
    expect:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx-deployment # added by apply
        annotations:
          # The annotation contains the updated image to nginx 1.11.9,
          # but does not contain the updated replicas to 2
          kubectl.kubernetes.io/last-applied-configuration: |
            {"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx-deployment"},"spec":{"selector":{"matchLabels":{"app":"nginx"}},"template":{"metadata":{"labels":{"app":"nginx"}},"spec":{"containers":[{"image":"nginx:1.11.9","name":"nginx","ports":[{"containerPort":80}]}]}}}}
      spec:
        selector:
          matchLabels:
            app: nginx
        replicas: 2
        # minReadySeconds cleared by apply
        template:
          metadata:
            labels:
              app: nginx
          spec:
            containers:
            - image: nginx:1.11.9 # Set by apply
              name: nginx
              ports:
              - containerPort: 80
  - name: identical strategic patch
    changed: false
    source:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx-deployment
    target:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx-deployment
    expect:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx-deployment
`)

func TestPatching(t *testing.T) {
	tests := updateChangedTestCases{}
	err := yaml.Unmarshal(testdata, &tests)
	if err != nil {
		t.Error(err)
		return
	}
	for _, test := range tests.TestCases {
		t.Run(test.Name, func(t *testing.T) {
			// original := fmt.Sprintf("%+v", test.Target)
			src := &unstructured.Unstructured{Object: test.Source}
			tgt := &unstructured.Unstructured{Object: test.Target}

			patch, err := NewPatch(src, tgt)
			if err != nil {
				t.Error(err)
			}

			if patch.IsRequired() != test.Changed {
				t.Errorf("IsRequired() = %v, expect: %v", patch.IsRequired(), test.Changed)
			}

			if patch.IsRequired() {
				patch.Merge(tgt)
				exp := &unstructured.Unstructured{Object: test.Expect}
				x, _ := tgt.MarshalJSON()
				y, _ := exp.MarshalJSON()
				if !bytes.Equal(x, y) {
					t.Errorf("\n     got %s\nexpected %s", string(x), string(y))
				}
			}
		})
	}
}
