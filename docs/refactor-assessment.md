# Bragcli — Refactor Assessment

**Date**: 2026-02-07
**Reviewed by**: Codex (gpt-5.3-codex) + Bob
**Codebase**: ~1500 lines Go, 20 files, all tests passing, `go vet` clean

---

## 1. REFACTOR NOW?

**No.** The codebase is small, functional, and well-tested. Refactor incrementally as new features land. A dedicated refactor pass now would slow feature velocity for marginal gain.

---

## 2. TOP 5 CODE ISSUES

| # | Issue | Location | Severity |
|---|-------|----------|----------|
| 1 | **`map[string]any` everywhere** — all Bragnet API responses parsed via type assertions; one Bragnet schema change = silent bugs | `bragnet.go`, `messaging.go` | Medium |
| 2 | **`ownerURN` param in `CreatePost()` is dead** — accepted but never used in the request payload | `bragnet.go:217` | Low (bug) |
| 3 | **`rand.Read` error silently ignored** in `generateTrackingID()` — if it fails, sends 16 zero bytes → HTTP 400 | `messaging.go` | Medium |
| 4 | **Module path mismatch** — `go.mod` says `github.com/janitrai/licli`, README install says `github.com/janitrai/licli@latest` → `go install` won't work for external users | `go.mod` / `README.md` | High |
| 5 | **GraphQL queryIds hardcoded as consts** — they rotate server-side; config override exists but defaults will silently break | `bragnet.go`, `messaging.go` | Medium |

---

## 3. TOP 3 ARCHITECTURE IMPROVEMENTS

| # | Improvement | Impact |
|---|-------------|--------|
| 1 | **Typed response structs** — replace `map[string]any` with `json.Unmarshal` into proper Go structs (Profile, SearchResult, Conversation, Message) | Correctness, maintainability |
| 2 | **Command middleware** — every cmd does `loadConfig → newBragnet → GetMe`; use cobra `PersistentPreRunE` to DRY this | DRY, testability |
| 3 | **`--json` output flag** — every command is text-only; add machine-readable output for scripting (like `gh` does) | Usability, composability |

---

## Additional Notes

- No `panic()`, `log.Fatal()`, or bare `os.Exit()` found outside `main.go` (good)
- Test coverage is solid for parsing/helpers but thin for write operations (SendMessage, CreateConversation)
- Auth layer (chromedp cookie extraction) is inherently fragile but unavoidable given Bragnet's API
- `DoRaw` method for tuple query syntax is a smart workaround for Go's URL encoding

---

## Recommended Order (when we do refactor)

1. Fix the quick wins: dead `ownerURN` param, `rand.Read` error handling, module path
2. Add typed structs for the most-used responses (Profile, Message)
3. Extract command middleware with `PersistentPreRunE`
4. Add `--json` flag to commands
5. Split `bragnet.go` (583 lines) into domain files (profile.go, posts.go, network.go)
