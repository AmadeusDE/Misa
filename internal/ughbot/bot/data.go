package bot

import "github.com/AmadeusDE/Misa/internal/ugh/llm"

type textData struct {
	Text string `json:"text"`
}

type mailData struct {
	From string `json:"from"`
	Text string `json:"text"`
}

type errorData struct {
	Message string `json:"message"`
}

type turnEndData struct {
	Text  string    `json:"text"`
	Usage llm.Usage `json:"usage"`
}

type toolCallData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Args string `json:"args"`
}

type toolResultData struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Output string `json:"output"`
	Error  bool   `json:"error,omitempty"`
}

type taskData struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

const clipBytes = 4000

func clip(s string) string {
	if len(s) <= clipBytes {
		return s
	}
	return s[:clipBytes] + "…"
}
