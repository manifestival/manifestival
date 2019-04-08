package yaml

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	olm = flag.Bool("olm", false,
		"Ignores resources managed by the Operator Lifecycle Manager")
	Recursive = flag.Bool("recursive", false,
		"If filename is a directory, process all manifests recursively")
	log = logf.Log.WithName("manifests")
)

type YamlManifest struct {
	dynamicClient dynamic.Interface
	resources     []unstructured.Unstructured
}

func NewYamlManifest(pathname string, config *rest.Config) *YamlManifest {
	client, _ := dynamic.NewForConfig(config)
	log.Info("Reading YAML file", "name", pathname)
	result := &YamlManifest{resources: parse(pathname), dynamicClient: client}
	if *olm {
		return result.Filter(ByOLM)
	} else {
		return result
	}
}

func (f *YamlManifest) ApplyAll() error {
	for _, spec := range f.resources {
		if err := f.Apply(&spec); err != nil {
			return err
		}
	}
	return nil
}

func (f *YamlManifest) Apply(spec *unstructured.Unstructured) error {
	resource, err := f.resource(spec)
	if err != nil {
		return err
	}
	current, err := resource.Get(spec.GetName(), v1.GetOptions{})
	if err != nil {
		// Create new one
		if !errors.IsNotFound(err) {
			return err
		}
		log.Info("Creating", "type", spec.GroupVersionKind(), "name", spec.GetName())
		if _, err = resource.Create(spec, v1.CreateOptions{}); err != nil {
			return err
		}
	} else {
		// Update existing one
		log.Info("Updating", "type", spec.GroupVersionKind(), "name", spec.GetName())
		// We need to preserve the current content, specifically
		// 'metadata.resourceVersion' and 'spec.clusterIP', so we
		// only overwrite fields set in our resource
		content := current.UnstructuredContent()
		for k, v := range spec.UnstructuredContent() {
			if k == "metadata" || k == "spec" {
				m := v.(map[string]interface{})
				for kn, vn := range m {
					unstructured.SetNestedField(content, vn, k, kn)
				}
			} else {
				content[k] = v
			}
		}
		current.SetUnstructuredContent(content)
		if _, err = resource.Update(current, v1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (f *YamlManifest) DeleteAll() error {
	a := make([]unstructured.Unstructured, len(f.resources))
	copy(a, f.resources)
	// we want to delete in reverse order
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
	for _, spec := range a {
		if err := f.Delete(&spec); err != nil {
			return err
		}
	}
	return nil
}

func (f *YamlManifest) Delete(spec *unstructured.Unstructured) error {
	resource, err := f.resource(spec)
	if err != nil {
		return err
	}
	if _, err = resource.Get(spec.GetName(), v1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
	}
	log.Info("Deleting", "type", spec.GroupVersionKind(), "name", spec.GetName())
	if err = resource.Delete(spec.GetName(), &v1.DeleteOptions{}); err != nil {
		// ignore GC race conditions triggered by owner references
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (f *YamlManifest) DeepCopyResources() []unstructured.Unstructured {
	result := make([]unstructured.Unstructured, len(f.resources))
	for i, spec := range f.resources {
		result[i] = *spec.DeepCopy()
	}
	return result
}

func (f *YamlManifest) ResourceNames() []string {
	var names []string
	for _, spec := range f.resources {
		names = append(names, fmt.Sprintf("%s (%s)", spec.GetName(), spec.GroupVersionKind()))
	}
	return names
}

func (f *YamlManifest) resource(spec *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	groupVersion, err := schema.ParseGroupVersion(spec.GetAPIVersion())
	if err != nil {
		return nil, err
	}
	groupVersionResource := groupVersion.WithResource(pluralize(spec.GetKind()))
	return f.dynamicClient.Resource(groupVersionResource).Namespace(spec.GetNamespace()), nil
}

func parse(pathname string) []unstructured.Unstructured {
	in, out := make(chan []byte, 10), make(chan unstructured.Unstructured, 10)
	go read(pathname, in)
	go decode(in, out)
	result := []unstructured.Unstructured{}
	for spec := range out {
		result = append(result, spec)
	}
	return result
}

func read(pathname string, sink chan []byte) {
	defer close(sink)
	file, err := os.Stat(pathname)
	if err != nil {
		log.Error(err, "Unable to get file info")
		return
	}
	if file.IsDir() {
		readDir(pathname, sink)
	} else {
		readFile(pathname, sink)
	}
}

func readDir(pathname string, sink chan []byte) {
	list, err := ioutil.ReadDir(pathname)
	if err != nil {
		log.Error(err, "Unable to read directory")
		return
	}
	for _, f := range list {
		name := path.Join(pathname, f.Name())
		switch {
		case f.IsDir() && *Recursive:
			readDir(name, sink)
		case !f.IsDir():
			readFile(name, sink)
		}
	}
}

func readFile(filename string, sink chan []byte) {
	file, err := os.Open(filename)
	if err != nil {
		panic(err.Error())
	}
	manifests := yaml.NewDocumentDecoder(file)
	defer manifests.Close()
	buf := buffer(file)
	for {
		size, err := manifests.Read(buf)
		if err == io.EOF {
			break
		}
		b := make([]byte, size)
		copy(b, buf)
		sink <- b
	}
}

func decode(in chan []byte, out chan unstructured.Unstructured) {
	for buf := range in {
		spec := unstructured.Unstructured{}
		err := yaml.NewYAMLToJSONDecoder(bytes.NewReader(buf)).Decode(&spec)
		if err != nil {
			if err != io.EOF {
				log.Error(err, "Unable to decode YAML; ignoring")
			}
			continue
		}
		out <- spec
	}
	close(out)
}

func buffer(file *os.File) []byte {
	var size int64 = bytes.MinRead
	if fi, err := file.Stat(); err == nil {
		size = fi.Size()
	}
	return make([]byte, size)
}

func pluralize(kind string) string {
	ret := strings.ToLower(kind)
	switch {
	case strings.HasSuffix(ret, "s"):
		return fmt.Sprintf("%ses", ret)
	case strings.HasSuffix(ret, "policy"):
		return fmt.Sprintf("%sies", ret[:len(ret)-1])
	default:
		return fmt.Sprintf("%ss", ret)
	}
}
