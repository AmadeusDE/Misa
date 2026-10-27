package llm

// MetaSource is the Message.Meta key naming where a message's text came from.
const MetaSource = "source"

// Message sources. Tool output and recalled data are untrusted: they may carry
// instructions aimed at the model that the user never wrote.
const (
	SourceUser       = "user"
	SourceToolOutput = "tool_output"
	SourceHarness    = "harness"
)

// Source is where the message text came from: the recorded tag, else what the
// role implies (tool results are tool output, user messages are user text).
func (m Message) Source() string {
	if s := m.Meta[MetaSource]; s != "" {
		return s
	}
	switch m.Role {
	case RoleTool:
		return SourceToolOutput
	case RoleUser:
		return SourceUser
	}
	return ""
}

// WithSource returns m tagged with source, leaving m's own Meta untouched.
func (m Message) WithSource(source string) Message {
	meta := make(map[string]string, len(m.Meta)+1)
	for k, v := range m.Meta {
		meta[k] = v
	}
	meta[MetaSource] = source
	m.Meta = meta
	return m
}

// Untrusted reports whether the message carries output the user did not write.
func (m Message) Untrusted() bool { return m.Source() == SourceToolOutput }
