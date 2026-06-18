package pipeline

import "maps"

// outcomesEqual compares two outcomes for unit tests (including maps).
func outcomesEqual(got, want *ErrorOutcome) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}

	return errorOutcomeCorePathsEqual(got, want) && errorOutcomeContextFieldsEqual(got, want)
}

func errorOutcomeContextFieldsEqual(got, want *ErrorOutcome) bool {
	if !maps.Equal(got.StringFields, want.StringFields) {
		return false
	}

	if !maps.Equal(got.IntFields, want.IntFields) {
		return false
	}

	return stringArrayFieldsEqual(got.StringArrayFields, want.StringArrayFields)
}

//nolint:gocritic // hugeParam: tests compare full ErrorOutcome snapshots by field
func errorOutcomeCorePathsEqual(got, want *ErrorOutcome) bool {
	return got.Error == want.Error && got.Hint == want.Hint && got.ExitCode == want.ExitCode &&
		got.Src == want.Src && got.Dest == want.Dest && got.OutDir == want.OutDir && got.OutputPath == want.OutputPath
}

func stringArrayFieldsEqual(left, right map[string][]string) bool {
	if len(left) != len(right) {
		return false
	}

	for key, av := range left {
		bv, ok := right[key]
		if !ok || len(av) != len(bv) {
			return false
		}

		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
	}

	return true
}
