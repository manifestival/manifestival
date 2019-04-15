package manifestival

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Parse parses YAML files into Unstructured objects.
//
// It supports 4 cases today:
// 1. pathname = path to a file --> parses that file.
// 2. pathname = path to a directory, recursive = false --> parses all files in
//    that directory.
// 3. pathname = path to a directory, recursive = true --> parses all files in
//    that directory and it's descendants
// 4. pathname = url --> fetches the contents of that URL and parses them as YAML.
func Parse(pathname string, recursive bool) ([]unstructured.Unstructured, error) {
	if isURL(pathname) {
		return parseURL(pathname)
	}

	return parseTree(pathname, recursive)
}

// parseFile parses a single file.
func parseFile(pathname string) ([]unstructured.Unstructured, error) {
	file, err := os.Open(pathname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return parse(file)
}

// parseTree parses a whole tree of files. Descendant directories will be ignored
// if the recursive flag is set to false.
func parseTree(root string, recursive bool) ([]unstructured.Unstructured, error) {
	aggregated := []unstructured.Unstructured{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip directories if no recursive behavior is wanted.
			if path != root && !recursive {
				return filepath.SkipDir
			}
			return nil
		}

		els, err := parseFile(path)
		if err != nil {
			return err
		}
		aggregated = append(aggregated, els...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return aggregated, nil
}

// parseURL fetches a URL and parses its contents as YAML.
func parseURL(url string) ([]unstructured.Unstructured, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parse(resp.Body)
}

// parse consumes the given reader and parses its contents as YAML.
func parse(reader io.Reader) ([]unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLToJSONDecoder(reader)
	objs := []unstructured.Unstructured{}
	var err error
	for {
		out := unstructured.Unstructured{}
		err = decoder.Decode(&out)
		if err != nil {
			break
		}
		objs = append(objs, out)
	}
	if err != io.EOF {
		return nil, err
	}
	return objs, nil
}

// isURL checks whether or not the given path parses as a URL.
func isURL(pathname string) bool {
	_, err := url.ParseRequestURI(pathname)
	return err == nil
}
