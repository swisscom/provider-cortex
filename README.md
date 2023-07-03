# provider-cortex

`provider-cortex` is a minimal [Crossplane](https://crossplane.io/) Provider for the [Cortex HTTP API](https://cortexmetrics.io/docs/api/). It comes
with the following features:

- A `ProviderConfig` type which allows setting the [X-Scope-OrgID](https://cortexmetrics.io/docs/api/#authentication) header which is required for Cortex' tenant model
- A `RuleGroup` resource type which implements the [RuleGroup API](https://cortexmetrics.io/docs/api/#get-rule-groups-by-namespace)
- An `AlertManagerConfig` resource type which implements the [Alertmanager API](https://cortexmetrics.io/docs/api/#get-alertmanager-configuration)


## Developing

1. Run `make submodules` to initialize the "build" Make submodule we use for CI/CD.
4. Add your new type by running the following command:
```
make provider.addtype provider=cortex group=alerts kind=AlertManagerConfiguration
```
3. Implement types and controller
5. Run `make reviewable` to run code generation, linters, and tests.
5. Run `make build` to build the provider.

Refer to Crossplane's [CONTRIBUTING.md] file for more information on how the
Crossplane community prefers to work. The [Provider Development][provider-dev]
guide may also be of use.

[CONTRIBUTING.md]: https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md
[provider-dev]: https://github.com/crossplane/crossplane/blob/master/contributing/guide-provider-development.md
