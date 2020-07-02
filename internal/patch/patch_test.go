package patch_test

import (
	"bytes"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	. "github.com/manifestival/manifestival/internal/patch"
)

type updateChangedTestCases struct {
	TestCases []updateChangedTestCase
}

type updateChangedTestCase struct {
	Name     string
	Changed  bool
	Modified map[string]interface{}
	Current  map[string]interface{}
	Expect   map[string]interface{}
}

var testdata = []byte(`
testCases:
  - name: identical maps
    changed: false
    modified:
      kind: T
      x:
        z: i
    current:
      kind: T
      x:
        z: i
    expect:
      kind: T
      x:
        z: i
  - name: add nested map entry
    changed: true
    modified:
      kind: T
      x:
        z: i
    current:
      kind: T
      x:
        a: foo
    expect:
      kind: T
      x:
        z: i
        a: foo
  - name: change nested map entry
    changed: true
    modified:
      kind: T
      x:
        z: i
    current:
      kind: T
      x:
        z: j
    expect:
      kind: T
      x:
        z: i
  - name: change missing map entry
    changed: true
    modified:
      kind: T
      x:
        z: i
    current:
      kind: T
    expect:
      kind: T
      x:
        z: i
  - name: identical nested slice
    changed: false
    modified:
      kind: T
      x:
        z:
          - i
          - j
    current:
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
    modified:
      kind: T
      x:
        z:
          - i
          - j
    current:
      kind: T
      x:
        z:
          - i
    expect:
      kind: T
      x:
        z:
          - i
          - j
  - name: update nested slice entry
    changed: true
    modified:
      kind: T
      x:
        z:
          - i
          - j
          - k
    current:
      kind: T
      x:
        z:
          - m
          - n
          - p
    expect:
      kind: T
      x:
        z:
          - i
          - j
          - k
  - name: add missing slice entry
    changed: true
    modified:
      kind: T
      x:
        z:
          - i
          - j
    current:
      kind: T
      x:
        x:
          - j
    expect:
      kind: T
      x:
        z:
          - i
          - j
        x:
          - j
  - name: change map within list
    changed: true
    modified:
      kind: T
      x:
        z:
          - foo: bar
    current:
      kind: T
      x:
        z:
          - foo: bar
            one: "1"
    expect:
      kind: T
      x:
        z:
          - foo: bar
  - name: strategic patch # https://kubernetes.io/docs/tasks/manage-kubernetes-objects/declarative-config/#merge-patch-calculation
    changed: true
    modified:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx-deployment
        annotations:
          modified: "true"
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
    current:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        annotations:
          current: "true"
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
          modified: "true"
          current: "true"
          kubectl.kubernetes.io/last-applied-configuration: |
            {"apiVersion":"apps/v1","kind":"Deployment","metadata":{"annotations":{},"name":"nginx-deployment","namespace":"default"},"spec":{"minReadySeconds":5,"selector":{"matchLabels":{"app":"nginx"}},"template":{"metadata":{"labels":{"app":"nginx"}},"spec":{"containers":[{"image":"nginx:1.7.9","name":"nginx","ports":[{"containerPort":80}]}]}}}}
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
    modified:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx-deployment
    current:
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
			// original := fmt.Sprintf("%+v", test.Current)
			mod := &unstructured.Unstructured{Object: test.Modified}
			cur := &unstructured.Unstructured{Object: test.Current}

			patch, err := New(cur, mod)
			if err != nil {
				t.Error(err)
			}
			if (patch == nil) == test.Changed {
				t.Errorf("actual = %v, expect: %v", patch, test.Changed)
			}
			if patch != nil {
				patch.Merge(cur)
				exp := &unstructured.Unstructured{Object: test.Expect}
				x, _ := cur.MarshalJSON()
				y, _ := exp.MarshalJSON()
				if !bytes.Equal(x, y) {
					t.Errorf("\n     got %s\nexpected %s", string(x), string(y))
				}
			}
		})
	}
}
