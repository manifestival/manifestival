# Manifestival

[![Build Status](https://travis-ci.org/manifestival/manifestival.svg?branch=master)](https://travis-ci.org/manifestival/manifestival)

Manipulate unstructured Kubernetes resources loaded from a manifest.

Manifestival helps building operators by providing mechanisms to create/apply/delete resources from manifests.
That means, you can feed your YAML to manifestival in your operator similar to feeding your YAML to kubectl, but on runtime.

Manifestival also provides transform functionality to shape your resources before pushing them to Kubernetes. 

## Usage

Basic Usage

Create a manifest from YAML files in a directory:

```go
recursive  := True // or False
restConfig := getMyClientGoRestConfig() 

manifest, err := mf.NewManifest("/path/to/resources-dir", recursive, cfg)
if err != nil {
    log.Error(err, "Error creating the Manifest")
    return
}
```

Apply the resources in the manifest:

```go 
manifest.ApplyAll()

```

Iterate over the resources in the manifest, e.g. for post-checking for status of them:

```go 
for _, u := range manifest.Resources {
    // u is of type unstructured.Unstructured of "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    log.Print("Namespace", u.getNamespace(), "Kind", u.getKind(), "Name", u.GetName())
    // ... do more things here
}

```

Transform resources before applying them on Kubernetes:

```go
// custom transformer that prepends "my" to resources names
func myTransformer(owner Owner) Transformer {
	return func(u *unstructured.Unstructured) error {
		u.setName("my" + u.getName())
		return nil
	}
}

 
transforms := []manifestival.Transformer{
    // example bundled transformers in manifestival package
    manifestival.InjectOwner(ownerResource),
    manifestival.InjectNamespace(myNamespace),
    // example custom transformer
    myTransformer
}

// transform the resources  
manifest = manifest.Transform(transforms...)
// then apply them
manifest.ApplyAll()

```



Delete resources in the manifest from Kubernetes:

```go
import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
...
manifest.DeleteAll(&metav1.DeleteOptions{})
```


This library isn't much use without a `Client` implementation. You
have two choices:

- [client-go](https://github.com/manifestival/client-go-client)
- [controller-runtime](https://github.com/manifestival/controller-runtime-client)

## Development

    dep ensure -v
    go test -v ./...
