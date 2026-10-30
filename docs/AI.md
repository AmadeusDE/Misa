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

Builds with `-tags nough` leave the AI features out.
