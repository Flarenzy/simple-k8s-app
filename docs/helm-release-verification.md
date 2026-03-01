# Helm Release Verification

Release charts are published to:

- `oci://ghcr.io/flarenzy/charts/ipam`

The release workflow also uploads these GitHub Release assets:

- `ipam-<version>.tgz`
- `ipam-<version>.tgz.prov`
- `helm-release-public.asc`

To verify and install a signed release:

```bash
helm pull oci://ghcr.io/flarenzy/charts/ipam --version 1.2.3
# download ipam-1.2.3.tgz.prov and helm-release-public.asc from the GitHub Release assets
gpg --import helm-release-public.asc
gpg --export > ./pubring.gpg
helm verify --keyring ./pubring.gpg ipam-1.2.3.tgz
helm install ipam oci://ghcr.io/flarenzy/charts/ipam --version 1.2.3
```

The release workflow signs the chart package with classic Helm GPG provenance signing before pushing it to GHCR. `helm verify` checks both the signature on the provenance file and that the downloaded chart archive matches the signed digest.

## Values Validation

The chart now includes `values.schema.json`, so Helm validates override files during:

- `helm lint`
- `helm template`
- `helm install`
- `helm upgrade`

Validate an override file before install:

```bash
helm lint deploy/helm/ipam -f values-keycloak.yaml
```

If an override is invalid, Helm fails before rendering manifests. For example, this bad value:

```yaml
api:
  auth:
    enabled: "true"
```

produces a schema error like:

```text
- at '/api/auth/enabled': got string, want boolean
```

This catches wrong types, unknown keys, invalid enums, and missing required nested values early.
