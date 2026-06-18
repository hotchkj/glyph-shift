package pipeline

import "testing"

func TestIntentToOSFlags_MapsKnownIntents(t *testing.T) {
	t.Parallel()

	exclusive := intentToOSFlags(OutputCreateExclusive)
	replace := intentToOSFlags(OutputCreateOrReplace)
	appendF := intentToOSFlags(OutputAppend)
	unknown := intentToOSFlags(OutputWriteIntent(99))

	if exclusive == replace {
		t.Fatal("exclusive and replace must differ")
	}
	if exclusive == appendF {
		t.Fatal("exclusive and append must differ")
	}
	if replace == appendF {
		t.Fatal("replace and append must differ")
	}
	if replace != unknown {
		t.Fatalf("unknown intent should fall through to replace: got %#x want %#x", unknown, replace)
	}
}
