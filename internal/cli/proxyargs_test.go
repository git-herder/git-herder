package cli

import (
	"reflect"
	"testing"
)

func TestPrepareProxyArgsExamples(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      []string
		out     []string
		tail    []string
		all     []string
		help    bool
		version bool
		same    bool
		wantErr bool
	}{
		{
			name:    "remote version",
			in:      []string{"remote", "-v"},
			out:     []string{"remote", "-v"},
			version: true,
		},
		{
			name: "remote git verbose",
			in:   []string{"remote", "--", "-v"},
			out:  []string{"remote", "--", "-v"},
			tail: []string{"-v"},
		},
		{
			name: "fetch all stays",
			in:   []string{"fetch", "--all"},
			out:  []string{"fetch", "--all"},
			all:  []string{"--all"},
		},
		{
			name: "fetch all passed through",
			in:   []string{"fetch", "-A", "--", "--all"},
			out:  []string{"fetch", "-A", "--", "--all"},
			tail: []string{"--all"},
			all:  []string{"-A"},
		},
		{
			name: "pull rebase tail",
			in:   []string{"pull", "-w", "~/src", "--qn", "^api-", "--rebase", "origin", "main"},
			out:  []string{"pull", "-w", "~/src", "--qn", "^api-", "--", "--rebase", "origin", "main"},
			tail: []string{"--rebase", "origin", "main"},
		},
		{
			name: "flags before command",
			in:   []string{"--qn", "^api-", "pull", "--ff-only"},
			out:  []string{"pull", "--qn", "^api-", "--", "--ff-only"},
			tail: []string{"--ff-only"},
		},
		{
			name: "cluster starts tail",
			in:   []string{"pull", "-iA"},
			out:  []string{"pull", "--", "-iA"},
			tail: []string{"-iA"},
		},
		{
			name: "short with value",
			in:   []string{"push", "-w=~/src", "-e=continue"},
			out:  []string{"push", "-w=~/src", "-e=continue"},
		},
		{
			name: "tail keeps later flags",
			in:   []string{"pull", "--rebase", "-w", "~/src"},
			out:  []string{"pull", "--", "--rebase", "-w", "~/src"},
			tail: []string{"--rebase", "-w", "~/src"},
		},
		{
			name: "dashdash inside tail",
			in:   []string{"pull", "--rebase", "--", "-h"},
			out:  []string{"pull", "--", "--rebase", "--", "-h"},
			tail: []string{"--rebase", "--", "-h"},
		},
		{
			name: "all forms",
			in:   []string{"fetch", "--all", "-A", "--all=true"},
			out:  []string{"fetch", "--all", "-A", "--all=true"},
			all:  []string{"--all", "-A", "--all=true"},
		},
		{
			name: "all false then short",
			in:   []string{"fetch", "--all=false", "-A"},
			out:  []string{"fetch", "--all=false", "-A"},
			all:  []string{"-A"},
		},
		{
			name: "all disabled",
			in:   []string{"fetch", "--all=false"},
			out:  []string{"fetch", "--all=false"},
		},
		{
			name: "help after command",
			in:   []string{"pull", "-h", "--rebase"},
			out:  []string{"pull", "-h", "--", "--rebase"},
			tail: []string{"--rebase"},
			help: true,
		},
		{
			name:    "version before help",
			in:      []string{"pull", "-v", "-h"},
			out:     []string{"pull", "-v", "-h"},
			version: true,
		},
		{
			name: "help before version",
			in:   []string{"pull", "--help", "-v"},
			out:  []string{"pull", "--help", "-v"},
			help: true,
		},
		{
			name:    "missing value",
			in:      []string{"pull", "-w"},
			wantErr: true,
		},
		{
			name:    "value looks like flag",
			in:      []string{"pull", "--workspace", "--help"},
			wantErr: true,
		},
		{
			name:    "help does not hide missing value",
			in:      []string{"pull", "-h", "-w"},
			wantErr: true,
		},
		{
			name:    "bad bool",
			in:      []string{"fetch", "--all=maybe"},
			wantErr: true,
		},
		{
			name:    "missing value before command",
			in:      []string{"--qn", "--all", "pull"},
			wantErr: true,
		},
		{
			name: "not a proxy command",
			in:   []string{"merge", "--all"},
			same: true,
		},
		{
			name: "unknown flag before other command",
			in:   []string{"--qn", "x", "report"},
			same: true,
		},
		{
			name: "dashdash before command",
			in:   []string{"--", "pull"},
			same: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, call, err := PrepareProxyArgs(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.same {
				if call != nil || !reflect.DeepEqual(out, tc.in) {
					t.Fatalf("out=%v call=%v", out, call)
				}
				return
			}
			if call == nil {
				t.Fatal("missing call")
			}
			if !reflect.DeepEqual(out, tc.out) {
				t.Fatalf("out=%#v", out)
			}
			if !reflect.DeepEqual(call.Tail, tc.tail) {
				t.Fatalf("tail=%#v", call.Tail)
			}
			if !reflect.DeepEqual(call.AllTokens, tc.all) {
				t.Fatalf("all=%#v", call.AllTokens)
			}
			if call.Help != tc.help || call.Version != tc.version {
				t.Fatalf("help=%v version=%v", call.Help, call.Version)
			}
		})
	}
}

func TestFormatProxyNotice(t *testing.T) {
	t.Parallel()
	if got := FormatProxyNotice([]string{"--all", "-A"}); got != "Not passed to git: --all -A" {
		t.Fatalf("%q", got)
	}
	if got := FormatProxyNotice(nil); got != "Not passed to git: --all" {
		t.Fatalf("%q", got)
	}
}
