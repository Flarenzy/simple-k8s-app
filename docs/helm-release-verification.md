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
gpg --verify ipam-1.2.3.tgz.prov ipam-1.2.3.tgz
helm install ipam oci://ghcr.io/flarenzy/charts/ipam --version 1.2.3
```

The release workflow signs the chart package with classic Helm GPG provenance signing before pushing it to GHCR.
