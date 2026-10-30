# AI in Misa

Misa's AI features use the providers [ugh](UGH.md) is set up with. There is one list of providers, keys and
models: what you configure for one works for the other.

## Providers

Providers live in ugh's `~/.config/ugh/config.toml` as `[providers.<name>]` tables, keys in ugh's credential
store (or the provider's environment variable, which wins). See [Providers and credentials](UGH.md#providers-and-credentials).
Built in: `opencode-go`, `opencode-zen`, `openrouter` and the `codex` login.

Presets for OpenAI-compatible endpoints (the base URL and key variable are filled in for you):

| preset | base URL | key |
|---|---|---|
| openai | `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| gemini | `https://generativelanguage.googleapis.com/v1beta/openai` | `GEMINI_API_KEY` |
| anthropic | `https://api.anthropic.com/v1` | `ANTHROPIC_API_KEY` |
| deepseek | `https://api.deepseek.com/v1` | `DEEPSEEK_API_KEY` |
| groq | `https://api.groq.com/openai/v1` | `GROQ_API_KEY` |
| mistral | `https://api.mistral.ai/v1` | `MISTRAL_API_KEY` |
| xai | `https://api.x.ai/v1` | `XAI_API_KEY` |
| together | `https://api.together.xyz/v1` | `TOGETHER_API_KEY` |
| fireworks | `https://api.fireworks.ai/inference/v1` | `FIREWORKS_API_KEY` |
| cerebras | `https://api.cerebras.ai/v1` | `CEREBRAS_API_KEY` |
| ollama | `http://localhost:11434/v1` | none |
| lmstudio | `http://localhost:1234/v1` | none |
| llamacpp | `http://localhost:8080/v1` | none |

Models are named `provider/model`, for example `ollama/llama3.2`. Without a model chosen for a feature, ugh's
`[models] default` is used.

## The configurator

`aiconfig` (typed at the prompt, `misa aiconfig`, the main menu's "AI providers" or Settings > AI providers) opens a
full-screen configurator. It edits ugh's `config.toml` and credential store, so ugh sees every change, and `ai.yaml`
for what is Misa's. Keys are never displayed: only the mask (`sk-…abcd`), and errors are scrubbed.

Provider list (columns: provider, kind, key status, what it is used for, base URL):

| key | action |
|---|---|
| `up` `down` `pgup` `pgdn` `home` `end` | move (the mouse wheel scrolls, a click selects, a second click edits) |
| `a` | add a provider: pick a preset or "Custom (OpenAI-compatible)", fill the form, then enter the key (skipped for local providers and when the environment already has one) |
| `enter` or `e` | edit a config-defined provider (name is fixed) |
| `k` | set the key: masked input, pasting works, `enter` stores it |
| `K` | remove the stored key (an environment variable is never touched) |
| `d` | delete a config-defined provider and its key, after `y` to confirm; built-ins cannot be deleted |
| `t` | test with a tiny completion, using the provider's default/chat/help model or else the first model it lists |
| `m` | list models |
| `D` | defaults |
| `r` `?` `q` | reload, help, leave |

Key status is `env sk-…abcd` (the environment variable wins), `key sk-…abcd` (stored), `logged in`, `no key needed` (a
local provider), `no login` (codex, the OpenCode Console) or `missing`. `codex` and the Console are logins: run
`ugh login codex` (or `ugh login opencode-console`). Built-in providers take a key and a model but are not edited here.

Form fields: name, base URL, key environment variable, headers (`Name: value; Name2: value2`) and no-thinking model
prefixes (comma separated). `tab`/`shift+tab` or `up`/`down` move between fields, `enter` moves on and saves on the last,
`ctrl+s` saves from anywhere, `esc` cancels.

Models: the list the provider reports, `/` filters it (`enter` keeps the filter, `esc` clears it), `enter` makes the model
ugh's default, `c` the chat model, `h` the help model, `r` fetches again.

Defaults: the default model (ugh's `models.default`), the chat and help models (blank means the default), the web search
engine (`enter` cycles duckduckgo, searxng, off; searxng asks for its URL first) and the SearXNG URL. `x` clears a value.

## ai.yaml

Misa's own AI settings, found like the other Misa config files (next to the executable, then `~/.config/misa/`):

```yaml
models:
  default: openrouter/xiaomi/mimo-v2.6-pro   # every feature without its own entry
  chat: ollama/llama3.2
  help: opencode-go/mimo-v2.6-pro
  help_fallback: ollama/llama3.2             # tried when help keeps failing with 429/5xx
search:
  engine: duckduckgo        # duckduckgo (no key) | searxng | off
  searxng_url: http://localhost:8888
```

## aihelp: command help

Ask about a command, or about anything on the command line, and get a short answer in Markdown. The model can
read the local man page and TLDR page of a command and search the Arch Wiki, so it answers from the documentation
of what you have installed. It uses the model chosen for `help` in `ai.yaml` (else your default); run `aiconfig`
to pick one. Without one, `aihelp` says so and stops.

```sh
aihelp ls                         # describe a command
aihelp ls how do I sort by size   # a question about a command
aihelp how do I unpack a .tar.zst # a free question (the first word is not a command)
aihelp --model ollama/llama3.2 -n tar how to list contents
aihelp --raw rsync trailing slash # print the Markdown as is
aihelp --fix "git psuh" 1         # print one corrected command
aihelp --clear-cache
```

- On a terminal the answer is rendered (headings, code blocks) as it arrives, block by block; piped it is plain
  Markdown. `--raw` forces plain output.
- Answers are cached under `~/.cache/misa/aihelp/` (one file per sha256 of the model and the question). `-n` / `--no-cache`
  skips the lookup and refreshes the entry.
- A 429 or 5xx from the provider is retried with a growing delay (2 s, 4 s). If the model still fails, the model
  named by `models.help_fallback` in `ai.yaml` is tried.
- `--fix "<command>" [exit status]` prints the corrected command on stdout and the explanation on stderr, so
  `fixed=$(aihelp --fix "$cmd" $?)` works. Pipe the command's error output in on stdin to include it.
- The Arch Wiki tool needs the network; man and tldr pages are local (tldr pages download once).

Two places in Misa use the same helper:

- **Command correction.** When a command fails and no built-in rule knows the fix, the prompt shows
  `no known fix  [ctrl+f: ask AI]`. `ctrl+f` asks the model about that command and its output. The answer appears as the
  usual `fix → …` line (with the explanation underneath), and `ctrl+f` again runs it. Nothing is sent until you press it,
  and the hint only shows when a help model is chosen.
- **Help viewer.** In the TLDR and man page viewer, `a` opens a prompt: ask about the page you are reading. The question
  and the answer are added under the page; `esc` cancels the prompt.

Builds with `-tags nough` leave the AI features out.
