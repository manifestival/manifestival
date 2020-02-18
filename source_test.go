package manifestival_test

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"testing"

	. "github.com/manifestival/manifestival"
)

func TestFromReader(t *testing.T) {
	tests := []struct {
		name                string
		reader              io.Reader
		expectedApiVersions []string
	}{{
		name: "from_bytes",
		reader: bytes.NewReader([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 3
---
apiVersion: v1
kind: Service
spec:
  selector:
    app: MyApp
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9376
`)),
		expectedApiVersions: []string{"apps/v1", "v1"},
	}}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s", tc.name), func(t *testing.T) {
			m, err := ManifestFrom(Reader(tc.reader))
			if err != nil {
				t.Fatalf("FromReader returned: %v", err)
			}

			foundApiVersions := make([]string, 0)
			for _, r := range m.Resources {
				foundApiVersions = append(foundApiVersions, r.GetAPIVersion())
			}
			if !reflect.DeepEqual(tc.expectedApiVersions, foundApiVersions) {
				t.Fatalf("Expected API kinds %v but found %v", tc.expectedApiVersions, foundApiVersions)
			}
		})
	}
}
