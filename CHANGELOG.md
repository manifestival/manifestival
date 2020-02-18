# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2019-02-17

### Changed

- Factored client API calls into a `Client` interface; implementations
  for [controller-runtime] and [client-go] reside in separate repos
  within this org (#4)
- Introduced `Option` and `ClientOption` types, enabling golang's
  "functional options" pattern for both Manifest creation and `Client`
  interface options, respectively (#6)
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
