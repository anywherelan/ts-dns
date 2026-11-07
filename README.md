# ts-dns

This is a fork of https://github.com/tailscale/tailscale, extracting out only net/dns.OSConfigurator and its dependencies.

The main motivation is to reduce go.mod dependencies and remove a few tailscale-specific bits.

## Sync source with upstream

```
go -C cmd/import-ts-dns build -o ../../import-ts-dns . && ./import-ts-dns; rm ./import-ts-dns
```

The script clones upstream, copies the needed packages, replaces some of them with stubs from
`cmd/import-ts-dns/files_replacement` and checks that everything builds (for linux, windows, darwin, freebsd),
passes `go vet` and tests. Only if the checks pass, the previously generated dirs are removed and replaced with the new output.

Only packages reachable from non-test code of `net/dns` are copied (`go list -deps ./net/dns` for each GOOS shows them),
plus `feature/linuxdnsfight`.
Upstream tests are kept only for `net/dns`, other packages are copied verbatim and tested upstream.

## Usage notes

- The names the configurators use in the OS are different from upstream, so they don't collide with
  tailscale installed on the same machine: the resolv.conf header and backup file, resolvconf and openresolv
  records, macOS resolver files and SystemConfiguration key, the Windows hosts file section, NRPT and policy
  registry keys. Config left with upstream names is treated as foreign.

- On linux in direct mode (plain `/etc/resolv.conf`, no systemd-resolved or NetworkManager) the configurator
  can watch the file and report when another program (e.g. a DHCP client) overwrites it.
  The watcher is enabled by importing `_ "github.com/anywherelan/ts-dns/feature/linuxdnsfight"`.
  The import works on any OS, outside linux the package is empty.
  It logs the overwrite and publishes `dns.TrampleDNS` on the `eventbus.Bus` passed to `NewOSConfigurator`;
  subscribe to it with `eventbus.SubscribeFunc` and call `SetDNS` again to restore the config.
  Upstream does the same in `dns.Manager`, which is not part of this fork.
- `NewOSConfigurator` panics on windows if the policy client is nil, pass `policyclient.NoPolicyClient{}`.

If upstream changed and the build fails, fix the script and run it again with `-keep`:
the upstream clone is then kept in `temp-clone-dir` and reused on the next run instead of cloning again.
Remove `temp-clone-dir` when done.
