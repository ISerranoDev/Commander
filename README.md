<p align="center"><img src="logos/logo.png" width="128" alt="Commander"/></p>

# Commander

Open-source SSH host manager, an alternative to Termius. Built with [Wails](https://wails.io) (Go + web UI).

All hosts and credentials are stored **locally**, in a single encrypted file. No account, no cloud.

## Security model

- **Master password** chosen on first run. It is never stored.
- Key derivation: **Argon2id** (t=3, m=64 MiB, p=4), random 16-byte salt.
- Encryption: **XChaCha20-Poly1305** (authenticated; any tampering is detected).
- The vault file is opaque: `salt || nonce || ciphertext`, with no magic bytes,
  version marker or visible parameters. The plaintext is padded to 4 KiB
  blocks, so the file size only reveals a rough upper bound of its content.
- Writes are atomic (temp file + fsync + rename) and the file is `0600`.
- Secrets never reach the UI: lists show only whether a password/key is set.
  When editing, an empty password field keeps the stored one.
- The vault locks automatically after inactivity (configurable, 15 min by
  default) and on exit. Locking wipes the derived key from memory.
- SSH private keys are validated before being stored (a `.pub` file or a
  wrong key passphrase is rejected). The key **contents** are copied into the
  vault; the original file is not referenced afterwards.
- Stored passwords can be revealed on demand (eye button); they are fetched
  from the vault only at that moment.
- **Server identity (TOFU):** on first connection the server key fingerprint
  is shown for confirmation and then stored in the vault. A changed key raises
  a warning. Hosts already trusted in `~/.ssh/known_hosts` are accepted
  without asking (that file is only read, never modified).

Existing data in the old `WailsCommander` folder is moved to `Commander`
automatically on first run.

Non-secret preferences (language, auto-lock) live in `settings.json` next to
the vault, because the language is needed before unlocking.

Vault location (override with `COMMANDER_DATA_DIR`):

| OS      | Path                                              |
|---------|---------------------------------------------------|
| macOS   | `~/Library/Application Support/Commander/store.dat` |
| Windows | `%AppData%\Commander\store.dat`              |
| Linux   | `~/.config/Commander/store.dat`              |

### Export / import

In **Settings → Data**:

- **Export**: a native "Save as" dialog asks where to save the backup
  (`.commander`), then a passphrase to encrypt it (independent of the master
  password; same opaque format as the vault).
- **Import**: a native "Open" dialog asks for the backup file, then its
  passphrase (asked again if wrong). Hosts are merged: entries with the same
  ID are replaced, new ones are added.

The chosen path is kept in Go; the UI cannot make the app read or write
arbitrary paths.

### Migrating from older versions

Old versions stored `hosts.json` in plaintext next to the executable. On first
run the app offers to import it into the vault and then delete it.

## Project layout

```
main.go                  Wails entry point
internal/
  app/                   Methods bound to the frontend (thin layer)
  vault/                 Encrypted storage: crypto, file format, CRUD, export
  model/                 Domain types and validation
  sshkey/                Private key parsing/validation
  settings/              Preferences (language, auto-lock)
  config/                Per-OS data paths
  legacy/                Reader for the old plaintext hosts.json
frontend/
  index.html
  src/main.js            Bootstrap, lock/unlock routing, idle auto-lock
  src/views/             unlock, hosts (list + editor), settings, sessions (tabs + xterm.js)
  src/i18n/              Translations (en, es)
  src/lib/               DOM helpers, toasts, modal, icons, settings store
  src/styles/app.css     Theme and layout
build/                   Platform packaging (icons, plist, NSIS)
docker/                  Optional Linux build container (X11)
```

## Platforms

| OS | Requirements |
|----|--------------|
| macOS 11+ | — (universal binary: `make build-macos`) |
| Windows 10 / 11 | WebView2 runtime (preinstalled on Windows 11) |
| Linux | GTK 3 and WebKit2GTK 4.1: `sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev` (Ubuntu 22.04+, Debian 12+) or `sudo dnf install gtk3-devel webkit2gtk4.1-devel` (Fedora) |

## Development

Requirements: Go 1.23+, Node 18+, Wails CLI v2
(`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

```sh
make dev                  # hot reload (wails dev)
make test                 # backend tests
make build                # binary for the current OS in build/bin
make build-windows        # cross-compile a Windows .exe from any OS
make build-linux          # on Linux
make build-linux-docker   # Linux binary from macOS/Windows via Docker
```

On Linux distros that still ship WebKit2GTK 4.0 only, drop `-tags webkit2_41`.

Use a throwaway vault while developing:

```sh
COMMANDER_DATA_DIR=/tmp/commander-dev wails dev
```

The older Docker/X11 setup (running the GUI inside a container) is in
[docker/README.md](docker/README.md).

## SSH sessions

Click a host to connect: a new tab opens with the connection progress and
then an [xterm.js](https://xtermjs.org) terminal. Output is streamed from Go
as Wails events (`ssh:data:<id>`), input goes back through `SendInput`.

Shortcuts. On Windows/Linux they use `Ctrl+Shift` so they never steal shell
keys (`Ctrl+W` deletes a word, `Ctrl+C` is SIGINT):

| Action | macOS | Windows / Linux |
|--------|-------|-----------------|
| Close tab | `Cmd+W` | `Ctrl+Shift+W` |
| Go to tab N (1 = vault) | `Cmd+1..9` | `Ctrl+1..9` |
| Next / previous tab | `Cmd+Shift+]` / `[` | `Ctrl+Tab` / `Ctrl+Shift+Tab` |
| Copy in terminal | `Cmd+C` | `Ctrl+Shift+C`, or `Ctrl+C` with a selection |
| Paste in terminal | `Cmd+V` | `Ctrl+Shift+V` |

## Translations

Strings live in `frontend/src/i18n/<lang>.js`. To add a language, copy
`en.js`, translate it, register it in `frontend/src/i18n/index.js` and add its
code to `Languages` in `internal/settings/settings.go` (plus the native dialog
titles in `internal/app/transfer.go`).

## Roadmap

- Split panes, snippets
- Jump hosts, port forwarding, ssh-agent support
- Tags
- SFTP

## Author

Developed by [ISerranoDev](https://github.com/ISerranoDev).

## License

Commander is source-available under the
[PolyForm Noncommercial License 1.0.0](LICENSE.md).

You may use, study, modify and share it for free for any **noncommercial**
purpose (personal use, research, education, charities, public institutions…).
Copies and derived works must keep the license and the copyright notice.

**Commercial use** (selling it, bundling it in a paid product or service, or
using it to make money) requires prior written permission from the author.
To ask for a commercial license, contact
[ISerranoDev](https://github.com/ISerranoDev).
