package pipeline

import (
	"path/filepath"

	"github.com/hotchkj/glyph-shift/internal/validate"
)

// ExtensionFromDeclaredOrSource returns declaredExt after validation when non-empty.
// When declaredExt is empty, derives filepath.Ext(srcPath); empty extension is accepted,
// otherwise the extension is validated via validate.ValidateExtension.
func ExtensionFromDeclaredOrSource(declaredExt, srcPath string) (string, error) {
	if declaredExt != "" {
		if err := validate.ValidateExtension(declaredExt); err != nil {
			return "", err
		}

		return declaredExt, nil
	}

	derived := filepath.Ext(srcPath)
	if derived == "" {
		return "", nil
	}

	if err := validate.ValidateExtension(derived); err != nil {
		return "", err
	}

	return derived, nil
}
