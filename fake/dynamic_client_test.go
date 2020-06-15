package fake_test

import (
	"testing"

	mf "github.com/manifestival/manifestival"

	"github.com/manifestival/manifestival/fake"
	"k8s.io/apimachinery/pkg/runtime"
	kFake "k8s.io/client-go/dynamic/fake"
)

func TestNewFakeDynamicClient(t *testing.T) {
	scheme := runtime.NewScheme()

	kfdc := kFake.NewSimpleDynamicClient(scheme)
	client := fake.NewFakeDynamicClient(kfdc)
	source := mf.Slice{}
	_, err := mf.ManifestFrom(source, mf.UseClient(client))
	if err != nil {
		t.Fatalf("received error %v", err)
	}
	// TODO: validate calls with objects in fake dynamic client
}
