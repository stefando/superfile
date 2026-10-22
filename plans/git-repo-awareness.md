# Git-repo awareness for superfile

## Context

When superfile is opened inside a git working tree (incl. linked worktrees), the user wants it to be repo-aware:
- start at the repo root (opt-in, only when no explicit path is given),
- copy file / PWD paths **relative to the repo root** (new hotkeys, existing absolute ones unchanged),
- show repo info (branch, later status) in the file panel.

Today there is zero git awareness in the codebase. Work is split into **3 independent PRs**, each shippable alone.

Decisions already made with the user:
- All 3 phases planned; status (phase 3) is opt-in.
- Start-at-root applies only when the path comes from `default_directory` (no CLI path).
- Two new hotkeys: relative path copy + relative PWD copy.

Root detection = walk up looking for a `.git` entry that is a **dir or file** (file = linked worktree / submodule). Verified empirically: matches `git rev-parse --show-toplevel` for normal repo, linked worktree, bare-repo worktree, and "bare repo itself → none". Known divergences (accepted): `GIT_DIR`/`GIT_WORK_TREE`/`core.worktree`, stale `.git` file of a deleted worktree.

---

## Phase 1 — root detection, start at root, relative copy (PR 1)

### 1a. New package `src/pkg/gitinfo` (no deps on `internal/`, like `src/pkg/utils`)
- `FindRoot(path string) (string, bool)`: lexical walk-up from `path` (`filepath.Dir` loop until it stops changing), `os.Lstat(filepath.Join(d, ".git"))` succeeds → return `d`. Lexical (no symlink resolution) so `filepath.Rel` against panel `Location` stays consistent.
- `RelToRoot(path string) string`: `FindRoot` + `filepath.Rel`; falls back to the absolute path when not in a repo or `Rel` errors. Root itself → `"."`.
- Tests `gitinfo_test.go` (table-driven, `t.TempDir()`, helpers `utils.SetupDirectories/SetupFiles` from `src/pkg/utils/test_utils.go`): plain repo (`.git` dir), worktree (`.git` file), nested submodule (inner `.git` file wins), outside any repo, path == root. No real `git` binary needed — just create `.git` dirs/files.

### 1b. Start at repo root
- `src/internal/common/config_type.go`: add next to `DefaultDirectory`
  `DefaultDirectoryGitRoot bool \`toml:"default_directory_git_root" comment:"..."\``
- `src/superfile_config/config.toml`: add documented `default_directory_git_root = false` under `default_directory`.
- `src/internal/config_function.go:97` `updateFirstFilePanelPaths`: record `fromDefault := firstPanelPaths[i] == ""` before substitution; after the path is resolved and `os.Stat` succeeds, if `fromDefault && common.Config.DefaultDirectoryGitRoot`, replace with `gitinfo.FindRoot(path)` when found. Zoxide/home fallbacks untouched.
- New test `src/internal/config_function_test.go`: table over (flag on/off) × (empty vs explicit path) × (in repo / not in repo); pass `zClient=nil`, set/restore `common.Config` fields.

### 1c. Relative copy hotkeys
- `config_type.go` hotkeys struct, after `CopyPWD`:
  `CopyPathRelativeToGitRoot []string \`toml:"copy_path_relative_to_git_root"\``
  `CopyPWDRelativeToGitRoot  []string \`toml:"copy_present_working_directory_relative_to_git_root"\``
- Defaults (validation in `common.LoadHotkeysFile` requires a non-empty first key; defaults fill missing fields, users just get the usual non-fatal "missing fields / --fix-hotkeys" note):
  - `src/superfile_config/hotkeys.toml`: `['C', '']`, `['ctrl+y', '']`
  - `src/superfile_config/vimHotkeys.toml`: same (`C` and `ctrl+y` are unbound in both).
- `src/internal/handle_file_operations.go` (next to `copyPath`/`copyPWD`, lines ~570–600): refactor minimally so both pairs share code:
  - `copyPathText()` gains a `relative bool` param (or a sibling `copyRelativePathText`); maps each line of `getSelectedOrFocusedPaths(panel)` through `gitinfo.RelToRoot`. Selected items are always within one panel dir, so one root lookup would suffice, but per-path `RelToRoot` keeps it trivially correct.
  - `copyRelativePath()` / `copyRelativePWD()` use `m.writeClipboard`.
- `src/internal/key_function.go`: add `case`s in `filePanelNormalModeKey` (both new keys) and `filePanelSelectModeKey` (path key only, mirroring existing `CopyPath` placement).
- `src/internal/ui/helpmenu/data.go`: two entries after `CopyPWD` (globalType).
- Tests in `src/internal/handle_file_operation_test.go`, new `TestCopyPathRelativeToGitRoot` mirroring `TestCopyPath` (same `captureClipboard` stub): focused file in repo subdir → `sub/file1.txt`; multi-select → newline-joined relatives; outside repo → absolute; PWD at root → `.`; PWD in subdir → `sub`.

### 1d. Docs
- `website/src/content/docs/configure/superfile-config.mdx`: `default_directory_git_root`.
- `website/src/content/docs/list/hotkey-list.mdx` (+ `configure/custom-hotkeys.mdx` if it lists the full default file): two new hotkeys.
- Translations (zh-tw etc.) left to maintainers unless they ask.

---

## Phase 2 — branch in the file panel, no subprocess (PR 2)

### 2a. `src/pkg/gitinfo` additions
- `type Info struct { Root, Repo, Branch string; Detached bool }`
- `Read(path string) (Info, bool)`:
  1. `FindRoot`.
  2. Resolve git dir: `.git` is dir → it; is file → parse `gitdir: <p>` (relative to root if not absolute).
  3. `HEAD` in git dir: `ref: refs/heads/<name>` → `Branch=<name>`; otherwise raw hash → `Branch=hash[:7]`, `Detached=true`.
  4. Repo name: if `<gitdir>/commondir` exists (linked worktree) resolve common dir → `Repo` = basename of its parent (or of itself minus `.git` if bare); else basename of `Root`.
- Tests: fabricate `.git/HEAD`, worktree layout (`gitdir:` + `commondir`), detached HEAD, missing/garbled HEAD → `ok=false`/empty branch, never panic.

### 2b. Panel state + refresh
- `src/internal/ui/filepanel/types.go`: add unexported `gitInfo gitinfo.Info` + `inGitRepo bool` to `Model`.
- `src/internal/ui/filepanel/get_elements.go` `UpdateElementsIfNeeded`: right after `m.element = m.getElements(...)`, refresh `m.gitInfo, m.inGitRepo = gitinfo.Read(m.Location)` (when the config flag is on). This piggybacks on the existing sync read cadence (forced on dir change, throttled otherwise), so branch switches done in another terminal show up without a watcher. Cost: a few stats + 1–3 tiny file reads.
- Accessor `GitInfo() (gitinfo.Info, bool)` for phase 3 and for 1c (optional: copy code can use the cached `Root` for the focused panel instead of re-walking; keep `RelToRoot` for simplicity).

### 2c. Render
- `src/internal/ui/filepanel/render.go` `Render`: file panels don't use `r.SetBorderTitle` today (`rendering/renderer_core.go:57`) → if `inGitRepo`, `r.SetBorderTitle(label)` with label `<git icon> <repo>:<branch>` (nerdfont) / `<repo>:<branch>` (plain); detached shows `@<hash>`. Path in top bar stays as-is.
- Add a branch icon constant in `src/config/icon` alongside existing ones.
- Config: `show_git_info = true` in `config_type.go` + `config.toml` + docs.
- Tests: extend filepanel render/unit tests to assert the title appears in a fake repo and is absent outside; flag off → absent.

---

## Phase 3 — async `git status` (PR 3, opt-in)

### 3a. `src/pkg/gitinfo/status.go`
- `type Status struct { Staged, Modified, Untracked, Conflicted, Ahead, Behind int; HasUpstream bool }`
- `GetStatus(root string, timeout time.Duration) (Status, error)`: run via existing `utils.ExecuteCommand(timeout, root, "git", "--no-optional-locks", "status", "--porcelain=v2", "--branch")` (`src/pkg/utils/shell_utils.go:27`; `--no-optional-locks` avoids touching the index lock while the user runs git). Parse:
  - `# branch.ab +A -B` → Ahead/Behind, HasUpstream.
  - `1 XY …` / `2 XY …` → X≠`.` Staged++, Y≠`.` Modified++.
  - `u …` → Conflicted++; `? …` → Untracked++.
  - `ExecuteCommand` returns combined output, so skip lines not matching the grammar (stderr warnings).
- `parseStatus(string) Status` split out and unit-tested with canned porcelain samples (no git binary needed). One integration test guarded by `exec.LookPath("git")` that `git init`s a temp repo.

### 3b. Wiring (copy the metadata async pattern, `src/internal/model.go:141` `getMetadataCmd` + `MetadataMsg` in `model_msg.go:141`)
- `model` gets `gitStatusCache map[string]gitStatusEntry{status, fetchedAt, inFlight}` keyed by **repo root** (shared across panels / subdirs).
- `getGitStatusCmd()` called in `Update` next to `getMetadataCmd()` (`model.go:108`): for each file panel with `GitInfo()` ok, if entry missing or older than TTL (5s) and not in flight → mark in flight, return `tea.Cmd` producing `GitStatusMsg{root, status, err, reqID: m.nextIoReqCnt()}`. Batch per distinct root.
- `GitStatusMsg.ApplyToModel`: store in cache, clear in-flight, push `SetGitStatus(status)` into every panel whose root matches.
- Invalidate (drop entry) after superfile's own mutations: in `PasteOperationMsg`/`CreateOperationMsg`/`DeleteOperationMsg` `ApplyToModel` (`model_msg.go:42–90`) and rename/compress/extract completions.
- `exec.ErrNotFound` → set `m.gitUnavailable = true`, log once, stop issuing. Timeout (2s) → keep last status, mark stale, retry after TTL.
- Render: append to phase-2 title, e.g. `main +2 ~3 ?1 ⇡1 ⇣2`, `✓` when clean (ASCII fallbacks without nerdfont). Truncation handled by the renderer.
- Config: `show_git_status = false`, docs.
- Tests: `GitStatusMsg` apply/ignore-stale tests in `src/internal` using `defaultTestModel`; cache TTL/in-flight logic unit-tested with an injectable status fetcher func on `model` (same idea as `clipboardWriter`).

---

## Verification (per phase)

- `make test` (`go test ./...`) and `make lint` (`golangci-lint run`, config `.golangci.yaml`) clean.
- Manual, using a scratch layout: `git init main; git -C main commit --allow-empty -m i; git -C main worktree add ../linked -b feat; mkdir -p linked/a/b`
  - Phase 1: in `linked/a/b` with `default_directory_git_root = true` → `spf` opens at `linked`; `spf .` stays in `a/b`. Press `C` on a file in `a/b` → clipboard `a/b/<file>`; `ctrl+y` → `a/b`; in `/tmp` both give absolute paths. Existing `ctrl+p`/`c` unchanged.
  - Phase 2: title shows `main:feat`; `git -C linked switch -c other` in another terminal → title updates within the refresh interval; navigate to `/tmp` → title disappears; navigate into `main` → `main:<default branch>`.
  - Phase 3: enable `show_git_status`; touch a file → `?1` appears within ~5s; stage it → `+1`; rename `git` on PATH (or `PATH=` minimal) → no status, no errors in UI, single log line.
- `spf --fix-hotkeys` / `--fix-config-file` add the new fields to an existing user config.

## Notes / risks
- Open upstream PR #1661 (OSC 52 clipboard) touches `copyPath`/`writeClipboard` → likely merge conflict for PR 1; rebase onto it if it lands first.
- No upstream issue exists for git integration; consider opening one before PR 3 to confirm maintainers want subprocess-based status.
