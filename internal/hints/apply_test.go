package hints

import "testing"

func TestApplyIncludeExcludeExtras(t *testing.T) {
	result, err := Apply(
		[]string{"gcc-native/13", "gcc-data/9.3", "openmpi/5", "gcc-native/13"},
		ModuleHints{
			Include:         []string{"gcc-native/13", "gcc-data/9.3"},
			ExcludePatterns: []string{"gcc-data/*"},
		},
		[]string{"aocc/4.2"},
	)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	assertStrings(t, result.Accepted, []string{"gcc-native/13", "aocc/4.2"})
	assertStrings(t, result.Extras, []string{"aocc/4.2"})
	if len(result.Rejected) != 2 {
		t.Fatalf("len(Rejected) = %d, want 2: %#v", len(result.Rejected), result.Rejected)
	}
	if result.Rejected[0].Module != "gcc-data/9.3" || result.Rejected[0].Reason != "excluded" || result.Rejected[0].Pattern != "gcc-data/*" {
		t.Fatalf("unexpected first rejection: %#v", result.Rejected[0])
	}
	if result.Rejected[1].Module != "openmpi/5" || result.Rejected[1].Reason != "not included" {
		t.Fatalf("unexpected second rejection: %#v", result.Rejected[1])
	}
}

func TestApplyRecordsMissingIncludes(t *testing.T) {
	result, err := Apply(
		[]string{"gcc-native/12"},
		ModuleHints{Include: []string{"gcc-native/13"}},
		nil,
	)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	assertStrings(t, result.Accepted, nil)
	assertStrings(t, result.MissingIncludes, []string{"gcc-native/13"})
	if len(result.Rejected) != 1 || result.Rejected[0].Reason != "not included" {
		t.Fatalf("unexpected rejections: %#v", result.Rejected)
	}
}

func TestApplyDoesNotDuplicateExtras(t *testing.T) {
	result, err := Apply([]string{"gcc-native/13"}, ModuleHints{}, []string{"gcc-native/13", "aocc/4.2"})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	assertStrings(t, result.Accepted, []string{"gcc-native/13", "aocc/4.2"})
	assertStrings(t, result.Extras, []string{"aocc/4.2"})
}

func TestApplyRejectsInvalidPattern(t *testing.T) {
	_, err := Apply([]string{"gcc-native/13"}, ModuleHints{ExcludePatterns: []string{"["}}, nil)
	if err == nil {
		t.Fatal("expected invalid pattern error")
	}
}

func TestApplyCleansWhitespaceAndDuplicates(t *testing.T) {
	result, err := Apply([]string{" gcc/13 ", "", "gcc/13", "openmpi/5"}, ModuleHints{}, []string{" ", "aocc/4.2"})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	assertStrings(t, result.Accepted, []string{"gcc/13", "openmpi/5", "aocc/4.2"})
}
