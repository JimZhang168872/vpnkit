# Upgrading vpnkit to v1.0.0

> **v1.0.0-rc.1 is a pre-release** with the full feature set landed. Treat it
> as production-ready for new installs; existing v0.10.x users must migrate.

## What's new

- **Multi-source subscriptions** — multiple订阅 coexist; nodes are selectable across all of them.
- **Local nodes** — hand-entered nodes (ss / vmess / trojan / vless / hysteria2 / tuic) live alongside subscription nodes.
- **Local rules** — structured CRUD via CLI and TUI.
- **Routing knobs** — explicit `mode` (rule / global / direct) and `global_target`.
- **New TUI layout** — Groups + Sources (Subscriptions / Local Nodes) + Local Rules + Routing tabs.

## Breaking change: store schema v1 → v2

v0.10.x stored a single active profile (`active_profile` + `[[profiles]]`).
v1.0.0 replaces that model with `[[subscriptions]]`, `[[local_nodes]]`, `[[local_rules]]`, `mode`, and `global_target`. Old store files are **not** auto-migrated.

On first launch under v1.0.0 the old store triggers a fatal:

```
store at ~/.config/vpnkit/config.toml uses schema v1 (vpnkit ≤ v0.10.x);
v1.0.0 changed the data model. Back up the file, then run
`vpnkit init --force` to regenerate
```

## Migration steps

1. **Back up subscriptions** (optional — `init --force` also saves a `.bak`):

   ```bash
   cp ~/.config/vpnkit/config.toml ~/vpnkit-v0.toml.bak
   ```

2. **Upgrade the binary**:

   ```bash
   vpnkit update          # if running v0.9+; auto-downloads v1.0.0-rc.1
   # or re-run install.sh
   ```

3. **Regenerate the store**:

   ```bash
   vpnkit init --force
   # ↳ moves old config to ~/.config/vpnkit/config.toml.bak.<timestamp>
   #   writes a fresh schema v2 file
   ```

4. **Re-add each subscription**:

   ```bash
   vpnkit subs add doge       https://example.invalid/sub/doge
   vpnkit subs add boost-net  https://example.invalid/sub/boost
   vpnkit subs update
   ```

5. **(Optional) Local nodes** for manually managed servers:

   ```bash
   vpnkit local-nodes add 'hysteria2://password@1.2.3.4:443?up=100&down=200#HK-manual'
   ```

6. **(Optional) Local rules** that override subscription rules:

   ```bash
   vpnkit local-rules add DOMAIN-SUFFIX baidu.com '🎯 Direct'
   vpnkit local-rules add DOMAIN-KEYWORD internal '🎯 Direct'
   ```

7. **Pick routing target**:

   ```bash
   vpnkit target doge-auto   # send the rules' MATCH to doge's url-test group
   ```

8. **Restart mihomo**:

   ```bash
   systemctl --user restart mihomo.service
   ```

## New CLI surface

```
vpnkit subs        list | add <name> <url> | rm <name> | enable <name> | disable <name> | update [<name>]
vpnkit local-nodes list | add <uri>        | rm <name> | edit <name> <key=val>...
vpnkit local-rules list | add <type> <payload> <target> | rm <idx> | move <from> <to>
vpnkit target      [<group-or-node>]      # show current or set
vpnkit mode        rule | global | direct
```

`vpnkit status` now prints subscriptions count, local nodes count, mode, and global target.

## What changed under the hood

- `internal/profiles/` is gone — replaced by `internal/app.Pipeline`.
- `internal/subscription/assemble.go` is gone — replaced by `internal/assembler/`.
- Four new leaf packages: `groups/`, `localnodes/`, `localrules/`, `assembler/`.
- Schema v2 lives in `internal/store/store.go`; v1 fields are kept as `Legacy*` aliases for detection only.

## Reporting issues

If something breaks during migration, attach `~/.config/vpnkit/config.toml.bak.*`
and the output of `vpnkit status` to the issue. v1.0.0-rc.1 is the first release
with this architecture and feedback on rough edges is most valuable now.
