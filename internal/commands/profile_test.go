package commands

import "testing"

func TestParseProfileNodeTypeSpec(t *testing.T) {
	cases := []struct {
		name        string
		spec        string
		wantName    string
		wantRole    string
		wantDesc    string
		wantRunner  string
		wantRunArgs []string
	}{
		{
			name:       "local login",
			spec:       "login=this:role=both:description=Login node",
			wantName:   "login",
			wantRole:   "both",
			wantDesc:   "Login node",
			wantRunner: "this",
		},
		{
			name:        "slurm gpu constraint",
			spec:        "gpu_compute_mi250x=srun:partition=gpu,constraint=mi250x:role=runtime",
			wantName:    "gpu_compute_mi250x",
			wantRole:    "runtime",
			wantRunner:  "srun",
			wantRunArgs: []string{"--partition=gpu", "--constraint=mi250x"},
		},
		{
			name:        "pbs runner",
			spec:        "cpu_compute=pbsdsh:n=0:role=runtime",
			wantName:    "cpu_compute",
			wantRole:    "runtime",
			wantRunner:  "pbsdsh",
			wantRunArgs: []string{"-n", "0"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseProfileNodeTypeSpec(tc.spec)
			if err != nil {
				t.Fatalf("parseProfileNodeTypeSpec returned error: %v", err)
			}
			if got.Name != tc.wantName {
				t.Fatalf("Name = %q, want %q", got.Name, tc.wantName)
			}
			if got.Role != tc.wantRole {
				t.Fatalf("Role = %q, want %q", got.Role, tc.wantRole)
			}
			if got.Description != tc.wantDesc {
				t.Fatalf("Description = %q, want %q", got.Description, tc.wantDesc)
			}
			if got.Runner.Kind != tc.wantRunner {
				t.Fatalf("Runner.Kind = %q, want %q", got.Runner.Kind, tc.wantRunner)
			}
			assertStringSlice(t, got.Runner.Args, tc.wantRunArgs)
		})
	}
}

func TestParseProfileNodeTypeSpecRejectsMissingRole(t *testing.T) {
	if _, err := parseProfileNodeTypeSpec("login=this"); err == nil {
		t.Fatal("expected missing role error")
	}
}

func TestParseProfileNodeTypeSpecsRejectsDuplicates(t *testing.T) {
	_, err := parseProfileNodeTypeSpecs([]string{
		"login=this:role=build_host",
		"login=this:role=runtime",
	})
	if err == nil {
		t.Fatal("expected duplicate node type error")
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
