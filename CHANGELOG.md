# Changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)

## [Unreleased]

### Added

- Introduced the `Source` interface, enabling the creation of a
  Manifest from any source [#11](https://github.com/manifestival/manifestival/pull/11)
- Added a `ManifestFrom` constructor to complement `NewManifest`,
  which now only works for paths to files, directories, and URL's
- Use `ManifestFrom(Recursive("dirname/"))` to create a manifest from
  a recursive directory search for yaml files.
- Use `ManifestFrom(Slice(v))` to create a manifest from any `v` of type
  `[]unstructured.Unstructured`
- Use `ManifestFrom(Reader(r))` to create a manifest from any `r` of
  type `io.Reader`
- Introduced a new `Filter` function in the `Manifestival` interface
  that returns a subset of resources matching one or more `Predicates`
- Convenient predicates provided: `NotCRDs`, `ByLabel`, and `ByGVK`

### Changed

- Removed the "convenience" functions from the `Manifestival`
  interface and renamed `ApplyAll` and `DeleteAll` to `Apply` and
  `Delete`, respectively. [#14](https://github.com/manifestival/manifestival/issues/14)
- The `Manifest` struct's `Client` is now public, so the handy `Get`
  and `Delete` functions formerly in the `Manifestival` interface can
  now be invoked directly on any manifest's `Client` member.
- Manifests created from a recursive directory search are now only
  possible via the new `ManifestFrom` constructor. The `NewManifest`
  constructor no longer supports a `recursive` option.
- Moved the path/yaml parsing logic into its own `sources` package to
  reduce the exported names in the `manifestival` package.
- Replaced the `ClientOption` type with `ApplyOption` and
  `DeleteOption`, adding `IgnoreNotFound` to the latter, thereby
  enabling `Client` implementations to honor it, simplifying delete
  logic for users invoking the `Client` directly. All `ApplyOptions`
  apply to both creates and updates
  [#12](https://github.com/manifestival/manifestival/pull/12)
- The `Manifest` struct's `Resources` member is no longer public.
  Instead, a `Manifest.Resources()` function is provided to return a
  deep copy of the manifest's resources, if needed.


## [0.1.0] - 2019-02-17

### Changed

- Factored client API calls into a `Client` interface; implementations
  for [controller-runtime] and [client-go] reside in separate repos
  within this org [#4](https://github.com/manifestival/manifestival/issues/4)
- Introduced `Option` and `ClientOption` types, enabling golang's
  "functional options" pattern for both Manifest creation and `Client`
  interface options, respectively [#6](https://github.com/manifestival/manifestival/issues/6)
- Transforms are now immutable, a feature developed in the old
  `client-go` branch
- Except for `ConfigMaps`, manifest resources are now applied using
  kubectl's strategic 3-way merging, a feature also developed in the
  old `client-go` branch. `ConfigMaps` use a JSON merge patch,
  which is essentially a "replace"
- Other small fixes from the old `client-go` branch have also been
  merged 


## [0.0.0] - 2019-01-11

### Changed

- This release represents the move from this project's original home,
  https://github.com/jcrossley3/manifestival.
- There were no releases in the old repo, just two branches: `master`,
  based on the `controller-runtime` client API, and `client-go`, based
  on the `client-go` API. 


[controller-runtime]: https://github.com/manifestival/controller-runtime-client
[client-go]: https://github.com/manifestival/client-go-client
[unreleased]: https://github.com/manifestival/manifestival/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/manifestival/manifestival/compare/v0.0.0...v0.1.0
[0.0.0]: https://github.com/manifestival/manifestival/releases/tag/v0.0.0
