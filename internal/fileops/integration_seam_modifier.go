//go:build integration

package fileops

// IntegrationModifierWriteAt writes through the modifier's underlying OS file handle. It exists only
// for integrations/fileops tests that must simulate in-handle tampering to assert CAS behavior.
// Not compiled into default builds.
func IntegrationModifierWriteAt(m *Modifier, p []byte, off int64) (int, error) {
	return m.file.WriteAt(p, off)
}
