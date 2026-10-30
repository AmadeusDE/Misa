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
search:
  engine: duckduckgo        # duckduckgo (no key) | searxng | off
  searxng_url: http://localhost:8888
```

## Chat

`aichat [-m provider/model] [conversation-id]` is a chat client in the terminal: typed at the prompt it takes the
pane over, `misa aichat …` runs it on the whole terminal, and the main menu has an "AI Chat" entry. It is not an agent:
the model has no file or shell tools. `-m` picks the model for this session; without it the conversation's own model is
used, then `models.chat` / `models.default` of `ai.yaml`, then ugh's `[models] default`. With none of those set the
header says `no model: run aiconfig`.

The screen has a conversation list on the left (hidden below 72 columns), the transcript, and a multi-line input.
Answers stream in as markdown; reasoning is shown as a dimmed, collapsed `▸ thinking` line. The header shows the
model, whether web search is on, and the tokens (and cost, when the provider reports it) of the conversation.

| key | action |
|---|---|
| `enter` | send |
| `alt+enter`, `ctrl+j` | newline |
| `esc` | stop the answer, cancel editing, leave the list |
| `up` (empty input) | edit and resend your last message (`esc` cancels) |
| `ctrl+r` | answer the last message again |
| `ctrl+y` | copy the last answer (host clipboard, else OSC 52) |
| `ctrl+t` | show or hide the thinking of every answer |
| `pgup` / `pgdown`, mouse wheel | scroll the transcript |
| `ctrl+n` | new conversation (keeps the model and web search choice) |
| `ctrl+b` | show or hide the conversation list |
| `tab` | move focus between the input and the list |
| `ctrl+o` | model picker: models of every provider with a key (or none needed), type to filter, `enter` picks; a typed `provider/model` works too |
| `ctrl+s` | system prompt of this conversation (`enter` saves, `alt+enter` newline) |
| `ctrl+g` | web search on or off |
| `ctrl+q`, `ctrl+d` (empty input) | leave |

In the list: `up`/`down` (or `k`/`j`) move, `enter` opens, `n` starts a new one, `r` renames, `d` deletes (then `y`
confirms). Clicking a row opens it. `ctrl+w` is Misa's multiplexer prefix and `alt+1-9`, `alt+h/j/k/l` and the alt
arrows switch panes, so the chat uses none of them. Inside Misa `ctrl+c` closes the pane; `ctrl+q` leaves the chat.

Conversations are JSON files in `~/.local/share/misa/chats/` (`$XDG_DATA_HOME/misa/chats/`), one per conversation:
title (the first line you wrote, until renamed), system prompt, model, messages, thinking and token totals. Only the
text of the turns is sent back to the model, not its thinking or tool calls.

### Web search

`ctrl+g` lets the model call two tools, `web_search(query)` and `fetch_url(url)`. Search is off in each new chat. Each
call appears as one dimmed line (`· searched: …`, `· fetched: …`) above the answer. Up to six model turns are used per
message. A model that refuses tools (HTTP 400 mentioning tools) gets one retry without them, noted in the transcript,
and search is switched off for that conversation.

```yaml
search:
  engine: duckduckgo        # duckduckgo (default, no key) | searxng | off
  searxng_url: http://localhost:8888
```

- `duckduckgo` reads the HTML results page of `html.duckduckgo.com`; no key, but it is a scraped page and DuckDuckGo may
  rate-limit or change it. 
- `searxng` queries `<searxng_url>/search?format=json`; the instance must allow the `json` format.
- `off` removes the tools, and `ctrl+g` says so.

`fetch_url` fetches http and https pages only, with a 15 second timeout, strips scripts, styles and markup, and returns
at most about 20 KB of text. It can reach any address the machine can, including local services; a page the model
reads can ask it to fetch more, so leave search off when you open pages you do not trust.

Builds with `-tags nough` leave the AI features out; `aichat` then prints `built without AI support (-tags nough)`.
