package transcript

import "strings"

// Fields holds the newest session fields of a transcript.
type Fields struct {
	AgentName, CustomTitle, PermissionMode, Model string
}

// Newest returns the newest non-empty agentName, customTitle, and permissionMode of the
// entries of path, and the newest model of a main-chain assistant entry whose model
// does not start with '<'.
func Newest(path string) (Fields, error) {
	var f Fields
	err := Each(path, func(e Entry) {
		keep(&f.AgentName, e.AgentName)
		keep(&f.CustomTitle, e.CustomTitle)
		keep(&f.PermissionMode, e.PermissionMode)
		if e.Type == "assistant" && !e.IsSidechain && e.Message != nil && !strings.HasPrefix(e.Message.Model, "<") {
			keep(&f.Model, e.Message.Model)
		}
	})
	return f, err
}

// keep sets *dst to v when v is not empty.
func keep(dst *string, v string) {
	if v != "" {
		*dst = v
	}
}
