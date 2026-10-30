package ai

// Preset is an OpenAI-compatible endpoint the configurator offers.
type Preset struct {
	Name, Label, BaseURL, KeyEnv, KeyURL string
	Local                                bool // runs on this machine and needs no key
}

// Presets lists the hosted and local providers the configurator offers besides ugh's built-ins.
func Presets() []Preset {
	return []Preset{
		{Name: "openai", Label: "OpenAI", BaseURL: "https://api.openai.com/v1", KeyEnv: "OPENAI_API_KEY", KeyURL: "https://platform.openai.com/api-keys"},
		{Name: "gemini", Label: "Google Gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", KeyEnv: "GEMINI_API_KEY", KeyURL: "https://aistudio.google.com/apikey"},
		{Name: "anthropic", Label: "Anthropic", BaseURL: "https://api.anthropic.com/v1", KeyEnv: "ANTHROPIC_API_KEY", KeyURL: "https://console.anthropic.com/settings/keys"},
		{Name: "deepseek", Label: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", KeyEnv: "DEEPSEEK_API_KEY", KeyURL: "https://platform.deepseek.com/api_keys"},
		{Name: "groq", Label: "Groq", BaseURL: "https://api.groq.com/openai/v1", KeyEnv: "GROQ_API_KEY", KeyURL: "https://console.groq.com/keys"},
		{Name: "mistral", Label: "Mistral", BaseURL: "https://api.mistral.ai/v1", KeyEnv: "MISTRAL_API_KEY", KeyURL: "https://console.mistral.ai/api-keys"},
		{Name: "xai", Label: "xAI", BaseURL: "https://api.x.ai/v1", KeyEnv: "XAI_API_KEY", KeyURL: "https://console.x.ai"},
		{Name: "together", Label: "Together AI", BaseURL: "https://api.together.xyz/v1", KeyEnv: "TOGETHER_API_KEY", KeyURL: "https://api.together.ai/settings/api-keys"},
		{Name: "fireworks", Label: "Fireworks AI", BaseURL: "https://api.fireworks.ai/inference/v1", KeyEnv: "FIREWORKS_API_KEY", KeyURL: "https://fireworks.ai/account/api-keys"},
		{Name: "cerebras", Label: "Cerebras", BaseURL: "https://api.cerebras.ai/v1", KeyEnv: "CEREBRAS_API_KEY", KeyURL: "https://cloud.cerebras.ai"},
		{Name: "ollama", Label: "Ollama (local)", BaseURL: "http://localhost:11434/v1", Local: true},
		{Name: "lmstudio", Label: "LM Studio (local)", BaseURL: "http://localhost:1234/v1", Local: true},
		{Name: "llamacpp", Label: "llama.cpp (local)", BaseURL: "http://localhost:8080/v1", Local: true},
	}
}
