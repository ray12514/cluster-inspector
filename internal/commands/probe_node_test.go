package commands

import "testing"

func TestParseNodeRunnerSpec(t *testing.T) {
	cases := []struct {
		name  string
		spec  string
		extra []string
		kind  string
		args  []string
	}{
		{
			name: "default this",
			spec: "",
			kind: "this",
		},
		{
			name: "srun key values",
			spec: "srun:partition=gpu,constraint=mi250x",
			kind: "srun",
			args: []string{"--partition=gpu", "--constraint=mi250x"},
		},
		{
			name:  "pbsdsh key values and raw extra",
			spec:  "pbsdsh:n=0",
			extra: []string{"-u"},
			kind:  "pbsdsh",
			args:  []string{"-n", "0", "-u"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := parseNodeRunnerSpec(tc.spec, tc.extra)
			if err != nil {
				t.Fatalf("parseNodeRunnerSpec returned error: %v", err)
			}
			if runner.Kind != tc.kind {
				t.Fatalf("Kind = %q, want %q", runner.Kind, tc.kind)
			}
			if len(runner.Args) != len(tc.args) {
				t.Fatalf("Args = %#v, want %#v", runner.Args, tc.args)
			}
			for i := range tc.args {
				if runner.Args[i] != tc.args[i] {
					t.Fatalf("Args[%d] = %q, want %q", i, runner.Args[i], tc.args[i])
				}
			}
		})
	}
}

func TestParseNodeRunnerSpecRejectsInvalid(t *testing.T) {
	if _, err := parseNodeRunnerSpec("ssh", nil); err == nil {
		t.Fatal("expected invalid runner error")
	}
	if _, err := parseNodeRunnerSpec("this", []string{"--partition=gpu"}); err == nil {
		t.Fatal("expected this runner args error")
	}
}
