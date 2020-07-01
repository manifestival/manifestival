package sources_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/manifestival/manifestival/pkg/sources"
)

func TestParsing(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		recursive bool
		want      []string
		wantError bool
	}{{
		name: "single directory",
		path: "testdata/tree",
		want: []string{"a", "b"},
	}, {
		name:      "single directory, recursive",
		path:      filepath.FromSlash("testdata/tree"),
		recursive: true,
		want:      []string{"foo", "bar", "baz", "a", "b"},
	}, {
		name:      "single file",
		path:      filepath.FromSlash("testdata/tree/dir/b.yaml"),
		recursive: true,
		want:      []string{"bar", "baz"},
	}, {
		name:      "single file, recursive",
		path:      filepath.FromSlash("testdata/tree/file.yaml"),
		recursive: true,
		want:      []string{"a", "b"},
	}, {
		name:      "missing file",
		path:      filepath.FromSlash("testdata/missing"),
		wantError: true,
	}, {
		name:      "dangling symlink",
		path:      filepath.FromSlash("testdata/dangling-symlink"),
		wantError: true,
	}, {
		name:      "dangling symlink",
		path:      filepath.FromSlash("testdata/dangling-symlink"),
		wantError: true,
	}, {
		name:      "absolute path",
		path:      filepath.Join(os.Getenv("PWD"), filepath.FromSlash("testdata/tree/dir/b.yaml")),
		recursive: true,
		want:      []string{"bar", "baz"},
	}, {
		name: "url",
		path: "https://raw.githubusercontent.com/manifestival/manifestival/master/testdata/tree/file.yaml",
		want: []string{"a", "b"},
	}, {
		name:      "missing url",
		path:      "http://thisurldoesntexistforsureimeanitreally.com/file.yaml",
		wantError: true,
	}, {
		name: "multiple urls",
		path: "https://raw.githubusercontent.com/manifestival/manifestival/master/testdata/tree/file.yaml,https://raw.githubusercontent.com/manifestival/manifestival/master/testdata/tree/dir/a.yaml",
		want: []string{"a", "b", "foo"},
	}, {
		name: "url and file",
		path: "https://raw.githubusercontent.com/manifestival/manifestival/master/testdata/tree/file.yaml,testdata/tree/dir/a.yaml",
		want: []string{"a", "b", "foo"},
	}, {
		name:      "url and directory, recursive",
		path:      "https://raw.githubusercontent.com/manifestival/manifestival/master/testdata/tree/file.yaml,testdata/tree/dir",
		recursive: true,
		want:      []string{"a", "b", "foo", "bar", "baz"},
	}, {
		name:      "file and directory, recursive",
		path:      filepath.FromSlash("testdata/tree/file.yaml") + "," + filepath.FromSlash("testdata/tree/dir"),
		recursive: true,
		want:      []string{"a", "b", "foo", "bar", "baz"},
	}, {
		name:      "empty -> invalid input",
		path:      "",
		wantError: true,
	}, {
		name:      "url and empty path -> invalid input",
		path:      "https://raw.githubusercontent.com/manifestival/manifestival/master/testdata/tree/file.yaml,",
		wantError: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := Parse(test.path, test.recursive)

			if err != nil && !test.wantError {
				t.Errorf("Parse() = %v, wanted no error", err)
			}

			if err == nil && test.wantError {
				t.Errorf("Expected an error from Parse()")
			}

			if len(actual) != len(test.want) {
				t.Errorf("Parse() = %v, want: %v", actual, test.want)
			}

			for i, spec := range actual {
				if spec.GetName() != test.want[i] {
					t.Errorf("Failed for '%s'; got '%s'; want '%s'", test.path, actual, test.want)
				}
			}
		})
	}
}
