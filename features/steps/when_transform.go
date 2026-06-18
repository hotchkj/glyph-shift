package steps

import "github.com/cucumber/godog"

func registerWhenTransform(sc *godog.ScenarioContext, tc *TestContext) {
	// When the caller transforms "<file>" to LF line endings
	sc.When(`^the caller transforms "([^"]*)" to LF line endings$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file, LineEndings: "lf"})
	})

	// When the caller transforms "<file>" to CRLF line endings
	sc.When(`^the caller transforms "([^"]*)" to CRLF line endings$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file, LineEndings: "crlf"})
	})

	// When the caller transforms "<file>" to CR line endings
	sc.When(`^the caller transforms "([^"]*)" to CR line endings$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file, LineEndings: "cr"})
	})

	sc.When(`^the caller transforms "([^"]*)" to invalid line endings$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file, LineEndings: "invalid"})
	})

	// When the caller transforms "<file>" trimming trailing whitespace
	sc.When(`^the caller transforms "([^"]*)" trimming trailing whitespace$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file, TrimTrailing: true})
	})

	// When the caller transforms "<file>" ensuring a final newline
	sc.When(`^the caller transforms "([^"]*)" ensuring a final newline$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file, FinalNewline: true})
	})

	// When the caller transforms "<file>" to LF line endings, trimming trailing whitespace, and ensuring a final newline
	sc.When(
		`^the caller transforms "([^"]*)" to LF line endings, trimming trailing whitespace, and ensuring a final newline$`,
		func(file string) error {
			return runTransformDirect(tc, transformDirectInput{
				Src: file, LineEndings: "lf", TrimTrailing: true, FinalNewline: true,
			})
		},
	)

	// When the caller transforms "<file>" with no operations
	sc.When(`^the caller transforms "([^"]*)" with no operations$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file})
	})

	// When the caller previews transforming "<file>" to LF line endings
	sc.When(`^the caller previews transforming "([^"]*)" to LF line endings$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file, LineEndings: "lf", Preview: true})
	})

	// When the caller previews transforming "<file>" trimming trailing whitespace
	sc.When(`^the caller previews transforming "([^"]*)" trimming trailing whitespace$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file, TrimTrailing: true, Preview: true})
	})

	// When the caller previews transforming "<file>" ensuring a final newline
	sc.When(`^the caller previews transforming "([^"]*)" ensuring a final newline$`, func(file string) error {
		return runTransformDirect(tc, transformDirectInput{Src: file, FinalNewline: true, Preview: true})
	})
}

// RegisterTransformWhen registers transform When step definitions.
func RegisterTransformWhen(sc *godog.ScenarioContext, tc *TestContext) {
	registerWhenTransform(sc, tc)
}
