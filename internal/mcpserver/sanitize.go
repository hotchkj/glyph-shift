package mcpserver

import (
	"path/filepath"
	"strings"
)

// normalizePathSeparatorsForCompare maps '\' to '/' so workspace roots and error paths match
// across platforms. filepath.ToSlash only substitutes the host OS separator; a Windows-style
// root string under Unix (for example Docker) keeps '\' literals while errors may use '/'.
func normalizePathSeparatorsForCompare(s string) string {
	return strings.ReplaceAll(s, `\`, `/`)
}

// sanitizedError replaces the visible error message while preserving the chain for errors.Is / Unwrap.
type sanitizedError struct {
	msg string
	err error
}

func (e *sanitizedError) Error() string {
	return e.msg
}

func (e *sanitizedError) Unwrap() error {
	return e.err
}

// sanitizeError returns an error whose message has workspaceRoot stripped to relative form,
// so absolute paths are not leaked to MCP clients. Other text is left intact when possible.
func sanitizeError(err error, workspaceRoot string) error {
	if err == nil {
		return nil
	}

	cleanRoot := filepath.Clean(workspaceRoot)
	if cleanRoot == "" || cleanRoot == "." {
		return err
	}

	msg := stripWorkspaceRootFromMessage(err.Error(), cleanRoot)
	if msg == err.Error() {
		return err
	}

	return &sanitizedError{msg: msg, err: err}
}

func stripWorkspaceRootFromMessage(msg, cleanRoot string) string {
	// Try exact match variants first (preserves message formatting when possible)
	for _, sep := range []string{string(filepath.Separator), "/"} {
		prefix := cleanRoot + sep
		msg = strings.ReplaceAll(msg, prefix, "")
	}

	normRoot := normalizePathSeparatorsForCompare(filepath.ToSlash(cleanRoot))
	if normRoot != "" {
		msg = strings.ReplaceAll(msg, normRoot+"/", "")
	}

	return stripCaseInsensitive(msg, cleanRoot)
}

// stripPrefixCaseInsensitive removes every case-insensitive occurrence of lowerPrefix from msg.
func stripPrefixCaseInsensitive(msg, lowerPrefix string) string {
	for {
		idx := strings.Index(strings.ToLower(msg), lowerPrefix)
		if idx < 0 {
			break
		}
		msg = msg[:idx] + msg[idx+len(lowerPrefix):]
	}

	return msg
}

// replaceBareRootWithDotCaseInsensitive replaces each case-insensitive occurrence of lowerRoot with ".".
func replaceBareRootWithDotCaseInsensitive(msg, lowerRoot string) string {
	for {
		idx := strings.Index(strings.ToLower(msg), lowerRoot)
		if idx < 0 {
			break
		}
		msg = msg[:idx] + "." + msg[idx+len(lowerRoot):]
	}

	return msg
}

// stripCaseInsensitive applies Windows-style case-insensitive workspace root stripping.
func stripCaseInsensitive(msg, cleanRoot string) string {
	slashRoot := normalizePathSeparatorsForCompare(filepath.ToSlash(cleanRoot))
	slashMsg := normalizePathSeparatorsForCompare(filepath.ToSlash(msg))
	lowerSlashMsg := strings.ToLower(slashMsg)
	lowerRoot := strings.ToLower(slashRoot)
	if lowerRoot != "" && strings.Contains(lowerSlashMsg, lowerRoot) {
		msg = slashMsg
		for _, sep := range []string{"/", "\\"} {
			msg = stripPrefixCaseInsensitive(msg, lowerRoot+sep)
		}

		return replaceBareRootWithDotCaseInsensitive(msg, lowerRoot)
	}

	msg = strings.ReplaceAll(msg, cleanRoot, ".")
	if slashRoot != cleanRoot {
		msg = strings.ReplaceAll(msg, slashRoot, ".")
	}

	return msg
}
