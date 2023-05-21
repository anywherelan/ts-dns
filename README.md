# ts-dns

This is a fork of https://github.com/tailscale/tailscale, extracting out only net/dns.OSConfigurator and its dependencies.

The main motivation is to reduce go.mod dependencies and remove a few tailscale-specific bits.

## Sync source with upstream

```
rm -rf version health envknob logtail atomicfile net types util AUTHORS go.mod go.sum LICENSE
cd cmd/import-ts-dns; go build; cd ../..; ./cmd/import-ts-dns/import-ts-dns
```