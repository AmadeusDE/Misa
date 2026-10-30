# AI in Misa: providers, chat, help

Three stacked pull requests. Each lives in its own worktree and branch:

| PR | Branch | Worktree | Base |
|----|--------|----------|------|
| 1. Provider configurator | `ai-providers` | `~/src/misa-ai-providers` | `master` |
| 2. AI chat | `ai-chat` | `~/src/misa-ai-chat` | `ai-providers` |
| 3. AI command help | `ai-help` | `~/src/misa-ai-help` | `ai-providers` |

PR 1 lands in two steps. Step 1a, the `internal/ai` package below, is committed first because PRs 2 and 3 build on it.
Step 1b is the configurator UI. PRs 2 and 3 must use only the 1a API. If they need more, they add it in their own
branch, in a separate file in `internal/ai`, so later rebases stay conflict-free.

## What exists

ugh already has the provider stack. Reuse it and do not write a second one.

- `internal/ugh/provider`: `ResolveIn(roots, "provider/model") (llm.Provider, model, error)`, `SpecsIn` (built-ins:
  opencode-go, opencode-zen, openrouter, console-discovered providers, plus config `[providers.<name>]`), `ListModels`,
  `Thinking`, `Pricing`, `ContextWindow`. `codex` is a login (ChatGPT OAuth), not a key.
- `internal/ugh/config`: `config.toml` (`PathIn(roots)`), `Provider{BaseURL, KeyEnv, Headers, SessionHeader,
  NoThinking, Usage}`, `Models.Default`, `Auth.Store` (auto|keyring|file). `validateProvider` has the rules.
- `internal/ugh/auth`: `OpenIn(roots, store)`, `Store.Get/Put/Remove/List`, `Resolve(provider, lookupEnv, store)`,
  `EnvVar`, `MaskKey`. Secrets never appear in errors, logs or the UI. Show `MaskKey` output only.
- `internal/ugh/llm`: `Provider.Stream(ctx, Request) (<-chan Event, error)`, `Message`, `ToolSpec`, `Event`.
- `cmd/ugh/main.go` `auth`/`login` subcommands show how keys are set and listed.
- Misa side: `internal/help` (man, tldr parsing, adapted from Gemhelp), `internal/thefuck`, `internal/config`
  (YAML, `findConfigFile`: next to the executable, then `~/.config/misa/`), in-process screens through
  `procmon.Viewer` (`m.enterProcMon(...)`, see `mines` in `internal/ui/app.go`).
- Gemhelp source, for PR 3: `~/src/Gemhelp/cmd/gemhelp/` (gemini.go tool loop, wiki.go, cache.go, fuck.go, format.go).
  It is the user's own project. Port freely; one line in `docs/CREDITS.md` is enough.

## Rules for all three PRs

- Read `docs/AGENTS.md` and `docs/GUIDELINES.md` first. TDD, small files, lipgloss only, `gofmt -l .` must print
  nothing, tests need no user config (use `t.TempDir()` + roots/env overrides, never the real `~/.config`).
- Dependency direction: `internal/ugh/*` must not import `internal/ai`. `internal/builtins` must not import
  `internal/ai` (builtins may not depend on ugh). Add both to `internal/ugh/deps_test.go`.
- Build tags: `internal/ai` imports ugh, so everything that uses it in `internal/ui` or `main` goes in a `!nough`
  file with a `nough` stub that prints "built without AI support (-tags nough)". Follow `internal/ui/agent_ugh.go`.
- Must pass before every commit: `go build ./... && go vet ./... && go test ./... && gofmt -l .`, plus
  `go build -tags nough -o /dev/null . && go vet -tags nough ./...` and
  `GOOS=darwin go vet ./... && GOOS=freebsd go vet ./...`. Tests must not hit the network: use `httptest` servers.
- Update `docs/AGENTS.md` (architecture tree and a Key Patterns bullet) and the user docs (`docs/UGH.md` for
  providers, plus a short `docs/AI.md` for Misa's AI features that each PR extends).
- Commit trailer: `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`. Do not push and do not open PRs.

## 1a. `internal/ai` (shared core)

`// Package ai gives Misa features one way to reach the AI providers ugh is configured with.`

```go
// Settings is Misa's ai.yaml (internal/config search order). Saved with SaveSettings.
type Settings struct {
    Models map[string]string `yaml:"models"` // feature -> "provider/model"; feature "default" falls back to ugh models.default
    Search Search           `yaml:"search"`
}
type Search struct {
    Engine     string `yaml:"engine"`      // "duckduckgo" (default, no key) | "searxng" | "off"
    SearxngURL string `yaml:"searxng_url"`
}
func LoadSettings() (Settings, error)
func SaveSettings(Settings) error

const FeatureChat, FeatureHelp = "chat", "help"

// ModelFor is the ref for a feature: Settings.Models[feature], else ugh's models.default, else an error that
// tells the user to run the configurator (`aiconfig`).
func ModelFor(feature string) (string, error)

// Client is one resolved provider/model.
type Client struct{ Ref, Model string; P llm.Provider }
func Open(ref string) (*Client, error)                  // provider.ResolveIn
func OpenFor(feature string) (*Client, error)           // ModelFor + Open
func (c *Client) Stream(ctx, llm.Request) (<-chan llm.Event, error) // fills Request.Model
func (c *Client) Complete(ctx context.Context, system string, msgs []llm.Message) (string, llm.Usage, error)

// Tool loop shared by chat (web search) and help (man/tldr/wiki).
type Tool struct {
    Spec llm.ToolSpec
    Run  func(ctx context.Context, args json.RawMessage) (string, error)
}
type Turn struct { // callbacks for streaming UIs; any may be nil
    OnText, OnThinking func(string)
    OnToolCall        func(name, args string)
    OnToolResult      func(name, result string, err error)
}
// RunTools streams turns, runs requested tools, feeds results back, until the model stops calling tools or
// maxSteps is reached. It returns the messages appended to msgs (assistant turns and tool results) and usage.
func (c *Client) RunTools(ctx context.Context, req llm.Request, tools []Tool, maxSteps int, t Turn) ([]llm.Message, llm.Usage, error)

// Provider administration, used by the configurator. Writes go to ugh's config.toml and credential store,
// so ugh sees every change.
type ProviderInfo struct {
    Name, BaseURL, KeyEnv string
    Kind      string // "builtin" | "custom" | "console" | "codex"
    KeySource string // "env" | "store" | "login" | "" (missing)
    KeyMasked string
    Custom    config.Provider
}
func Providers() ([]ProviderInfo, error)
func SaveProvider(name string, p config.Provider) error // validated like config.validateProvider
func RemoveProvider(name string) error                  // also removes its stored key
func SetKey(provider, key string) error
func RemoveKey(provider string) error
func SetDefaultModel(ref string) error                  // ugh models.default
func Models(ctx context.Context, provider string) ([]string, error)
func Test(ctx context.Context, ref string) (time.Duration, error) // tiny completion, short timeout

// Presets: OpenAI-compatible endpoints the configurator offers.
type Preset struct{ Name, Label, BaseURL, KeyEnv, KeyURL string; Local bool }
func Presets() []Preset
```

Presets (all OpenAI-compatible Chat Completions): openai `https://api.openai.com/v1`; gemini
`https://generativelanguage.googleapis.com/v1beta/openai`; anthropic `https://api.anthropic.com/v1`; deepseek;
groq; mistral; xai; together; fireworks; cerebras; ollama `http://localhost:11434/v1` (Local, no key);
lmstudio `http://localhost:1234/v1` (Local); llama.cpp `http://localhost:8080/v1` (Local). Built-ins (openrouter,
opencode-*) are listed by `Providers` already and are not presets. Check each URL against the provider's current docs
if unsure. A Local preset without a key must still resolve: verify ugh's key lookup allows an empty key for it, and
change ugh minimally if it doesn't.

Editing `config.toml`: users edit it by hand, so do not re-marshal the file. Replace or append only the
`[providers.<name>]` table (and `[providers.<name>.headers]`), and set `default` inside `[models]` textually,
keeping every other line and comment. Write atomically (temp file + rename, keep mode). Table-driven tests with
comments, other sections and a missing file.

Measure the binary size cost of `internal/ai` in a `-tags nough` build. Report it in the commit message.

As built (1a), additions that keep the signatures above:

- `ai.UghRoots` (ugh's `roots.Roots`, zero = the user's ugh) and `ai.SettingsPath` (empty = Misa config search order)
  are package variables. Tests use `aitest.Isolate(t)` (`internal/ai/aitest`), which points HOME and the XDG
  directories at temp dirs and clears every `*_API_KEY`; never write to the real config.
- `ModelFor` order: `Models[feature]`, `Models["default"]`, ugh `models.default`, else the `aiconfig` error.
- `ProviderInfo` also has `Local bool` (loopback base URL: no key needed).
- `RunTools`: `maxSteps` counts model turns (<= 0 means 10). The last turn is sent without tools so the model must
  answer; any call it still makes is dropped. A failing or unknown tool is reported to the model as `error: ...`.
- `SetDefaultModel("")` clears `models.default`. `SaveProvider`/`SetDefaultModel`/`RemoveProvider` parse the edited
  file back and refuse (writing nothing) when `config.toml` defines the entry in a form the text editor cannot
  change, such as an inline table.
- ugh changes: `auth.IsNoCredential`, `config.ValidateProvider`, `provider.ModelsIn`/`LookupBase`, no `Authorization`
  header for an empty key, and a missing key is not an error for a loopback provider. `internal/config` gained
  `FindFile` and `UserFile`.

## 1b. Configurator UI (PR 1)

An in-process screen, `aiconfig` command, also reachable from Settings ("AI providers") and the main menu.
Build it as its own package (`internal/aiconfig`, a `procmon.Viewer`, like `internal/mines`) so `internal/ui`
only gains the wiring.

- Provider list: name, kind, base URL, key status (env / stored `sk-…abcd` / login / missing), whether it's ugh's
  default. Keys: `a` add, `enter` edit, `k` set key (masked input, paste works), `K` remove key, `d` delete
  (confirm; built-ins can't be deleted), `t` test, `m` models, `?` help.
- Add: pick a preset or "Custom (OpenAI-compatible)", then a form with name, base URL, key env var, headers,
  no_thinking prefixes, then the key prompt (skipped for Local).
- Models view: fetched list, filterable. `enter` sets ugh's default, `c` the chat model, `h` the help model.
- Defaults panel: ugh default, chat, help, and the web search engine / SearXNG URL from `ai.yaml`.
- Codex shows login status only and says to run `ugh login`.
- Mouse: click to select, wheel to scroll (each screen handles mouse; see `docs/AGENTS.md`).
- Errors are shown inline and never include a key.

Tests drive the model with key messages against a temp ugh root and temp misa config dir.

## 2. AI chat (PR 2)

A chat client like Open WebUI or the ChatGPT/Claude apps, in the terminal. It is not an agent harness: no file or
shell tools. The command is `aichat [-m provider/model] [conversation-id]`, also on the main menu. Package
`internal/aichat` (`procmon.Viewer`).

- Layout: conversation sidebar (toggle `ctrl+b`, hidden when narrow), transcript, multi-line input
  (`enter` sends, `alt+enter`/`ctrl+j` newline). Header shows the model, token use and cost if reported.
- Streaming with markdown rendering (glamour v2 is already a dependency). Re-render during the stream at most
  every ~50 ms. Collapsible thinking, shown dimmed.
- Conversations are JSON in `~/.local/share/misa/chats/` (XDG data dir): new (`ctrl+n`), switch, rename, delete
  (confirm). Title is the first user line, truncated.
- Model picker (`ctrl+o`) lists models of configured providers via `ai.Providers`/`ai.Models`, filterable.
  Per-conversation system prompt (`ctrl+s`).
- Stop generation (`esc`), regenerate the last answer (`ctrl+r`), edit and resend the last user message (`up` on
  empty input), copy the last answer (`ctrl+y`: `internal/clipboard`, else OSC 52 the way the UI already does).
- Web search: toggle `ctrl+g` (`ctrl+w` is the mux prefix; check every key against the mux bindings), off by default, shown in the header. When on, the model gets `web_search(query)` and
  `fetch_url(url)` tools through `ai.RunTools`. DuckDuckGo HTML endpoint by default (no key), SearXNG JSON if
  configured, off disables it. `fetch_url` returns readable text (strip scripts/styles/tags, cap ~20 KB, http(s)
  only, 15 s timeout). Show tool calls in the transcript as one dimmed line ("searched: …"). If the model rejects
  tools (HTTP 400 mentioning tools), retry once without them and note it.
- Put search in `internal/ai/search.go` + `fetch.go` (new files) so PR 3 could reuse it later. Tests against
  `httptest` fixtures only.

## 3. AI command help (PR 3)

Gemhelp for Misa, on `internal/ai` instead of the Gemini SDK. Command `aihelp` (a multicall entry and in-process,
registered from a `!nough` package that `main` imports, never from `internal/builtins`):

- `aihelp <command> <question…>` and `aihelp <question…>`. Tools for the model: `man_page(name, section?)`
  and `tldr_page(name)` from `internal/help` (reuse; don't port Gemhelp's copies), `arch_wiki(query)` ported from
  Gemhelp's wiki.go. Port the system prompt from gemini.go and adapt it.
- Streams the answer, rendered as markdown for a terminal (glamour) and plain when stdout isn't a TTY.
  `--model ref`, `--no-cache`, `--raw`.
- Response cache keyed by sha256(model + query) under `~/.cache/misa/aihelp/` (port cache.go).
- `aihelp --fix "<failed command>" [exit code]`: Gemhelp's fuck.go mode. Prints one corrected command.
  Also wire it into Misa's thefuck flow: when no pattern rule matches a failed command, `ctrl+f` asks the AI
  (async, routed with `routeTo`, shown as the usual suggestion). Only when a help model resolves. Never automatic.
- Model fallback: if the call fails with 429/5xx, retry with backoff (Gemhelp's backoff), then try
  `Settings.Models["help_fallback"]` if set.
- Help screen integration: in the TLDR/man viewer (`internal/ui/helpscreen.go`), an "Ask AI" action (`a`) on the
  current page opens a prompt and shows the answer in the viewer.
