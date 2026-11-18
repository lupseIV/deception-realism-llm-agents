# wear: text store of the planted home

The environment layer (`../persona`, next to this folder) decides which files go
into the sandbox home, where, and with which timestamps. The text of those
files lives here and is embedded in the chamber binary (`go:embed`), so the
gateway image needs no extra files.

## Layout

```
<level>/<kind>/<name>[.tmpl]
```

| Level | Use |
|---|---|
| `low/` | Baseline home, planted at every level and in every condition, control included |
| `medium/` | Overrides or additions for environment realism medium (empty for now) |
| `high/` | Overrides or additions for environment realism high (empty for now) |

| Kind | Content | Planted as |
|---|---|---|
| `shell/` | `bashrc`, `bashrc_additions`, `profile`, `bash_logout`, `bash_aliases.tmpl`, `side_commands.tmpl` (history vocabulary, one command per line) | `~/.bashrc`, `~/.profile`, `~/.bash_logout`, `~/.bash_aliases`, lines of `~/.bash_history` |
| `git/` | `gitconfig.tmpl`, `ignore`, `remote.tmpl`, `repo_config.tmpl`, `info_exclude`, `description` | `~/.gitconfig`, `~/.config/git/ignore`, the project's `.git/config`, `.git/info/exclude`, `.git/description` |
| `ssh/` | `config.tmpl`, `known_hosts_names.tmpl` (one host per line) | `~/.ssh/config`, one hashed `~/.ssh/known_hosts` line per host |
| `editor/` | `vimrc`, `viminfo.tmpl` | `~/.vimrc`, `~/.viminfo` |
| `repl/` | `lesshst`, `python_history` | `~/.lesshst`, `~/.python_history` |
| `project/` | `meta.json`, `files/` | the persona's project under `~/work/<repo>`, as a git clone |

## Rules

- **Level fallback.** `wear.Open(level)` looks a name up in that level, then
  in each lower one (high, medium, low). A file at a higher level replaces
  the same path below it; a new path adds a file. For `project/files/` the
  folders are merged the same way.
- **Templates.** Files ending in `.tmpl` are Go `text/template` files. A
  missing field is an error. Fields: `FullName`, `User`, `Domain`, `Company`,
  `GitHost`, `Bastion`, `Repo`, `Remote`, `Marks` (list of `Index`, `Line`,
  `Path`, used by `viminfo.tmpl`). Other files are planted byte for byte.
- **List files** (`side_commands`, `known_hosts_names`, `remote`): one entry
  per line; empty lines and lines starting with `#` are skipped.
- **Project.** `project/meta.json` lists the files the history edits
  (`sources`), the persona's feature branches, the colleagues, and the
  commits oldest first (`author` 0 = the persona, n = colleague n; a persona
  commit names its `feature`; `flag` turns on lines in `files/`;
  `clonedAfter` = commits that existed at the persona's clone). In
  `files/`, a line starting with `?flag ` is kept once the commit with that
  flag is in the history, `?!flag ` only before it; a line `?flag` alone is a
  conditional empty line. The layer checks that every committed file exists
  and every file is committed, and the tests check that each commit changes
  exactly the files it lists.
- **No credentials.** Nothing under `wear/` may hold a credential or refer to
  AWS: tokens and their context belong to the honeytoken layers. The
  persona tests enforce this on the planted output.
