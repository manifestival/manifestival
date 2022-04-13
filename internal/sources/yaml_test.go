package sources_test

import (
	"strings"
	"testing"

	. "github.com/manifestival/manifestival/internal/sources"
)

func TestInvalidManifest(t *testing.T) {
    manifests  := "*%*%&$&#@(!)@#!#"
    reader := strings.NewReader(manifests)
    _, err := Decode(reader)
    if err == nil {
        t.Errorf("Invalid YAML should have errored")
    }
}
