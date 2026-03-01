# Helm Release Verification

Release charts are published to:

- `oci://ghcr.io/flarenzy/charts/ipam`

To verify and install a signed release:

```bash
helm pull oci://ghcr.io/flarenzy/charts/ipam --version 1.2.3
gpg --verify ipam-1.2.3.tgz.prov ipam-1.2.3.tgz
helm install ipam oci://ghcr.io/flarenzy/charts/ipam --version 1.2.3
```

The release workflow signs the chart package with classic Helm GPG provenance signing before pushing it to GHCR.
