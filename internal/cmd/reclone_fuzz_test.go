// SPDX-FileCopyrightText: 2025 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"
)

// FuzzSanitizeCmd checks the property reclone's log line depends on: a token
// given to a credential flag never survives sanitizeCmd, whatever surrounds it.
func FuzzSanitizeCmd(f *testing.F) {
	f.Add("ghorg clone org", "--token=", "s3cr3tT0ken", "--skip-forks")
	f.Add("ghorg clone org --match-regex=a-t=b", "--token=", "s3cr3tT0ken", "")
	f.Add("ghorg clone org --token=first1234", "-t ", "s3cr3tT0ken", "--token second")
	f.Add("ghorg clone org", "--bitbucket-api-token ", "s3cr3tT0ken", "-t")
	f.Add("ghorg clone 'org", "-t=", "s3cr3tT0ken", "x'")

	forms := []string{"--token=", "--token ", "-t=", "-t ", "--bitbucket-api-token=", "--bitbucket-api-token "}
	f.Fuzz(func(t *testing.T, prefix, form, token, suffix string) {
		if !isPlainToken(token) || strings.Contains(prefix+suffix, token) {
			t.Skip()
		}
		known := false
		for _, candidate := range forms {
			known = known || form == candidate
		}
		if !known {
			form = forms[len(form)%len(forms)]
		}

		for _, value := range []string{token, "'" + token + " x'", `"` + token + ` x"`} {
			cmd := prefix + " " + form + value + " " + suffix
			if got := sanitizeCmd(cmd); strings.Contains(got, token) {
				t.Fatalf("sanitizeCmd(%q) = %q, which still contains the token", cmd, got)
			}
		}
	})
}

// isPlainToken reports whether s looks like a real token: long enough not to
// occur by accident, and free of the spaces and quotes that delimit arguments.
func isPlainToken(s string) bool {
	if len(s) < 8 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// FuzzSplitCommandArgs checks that splitting never yields an empty argument,
// and that a command with no quotes splits exactly on its spaces.
func FuzzSplitCommandArgs(f *testing.F) {
	f.Add(`ghorg clone org --match-regex "(foo|bar)"`)
	f.Add(`ghorg clone org --match-regex '(foo|bar)' --skip-forks`)
	f.Add(`ghorg  clone   org`)
	f.Add(`ghorg clone "unterminated`)

	f.Fuzz(func(t *testing.T, cmd string) {
		args := splitCommandArgs(cmd)
		for i, arg := range args {
			if arg == "" {
				t.Fatalf("splitCommandArgs(%q)[%d] is empty: %q", cmd, i, args)
			}
		}
		if strings.ContainsAny(cmd, `'"`) {
			return
		}
		want := strings.FieldsFunc(cmd, func(r rune) bool { return r == ' ' })
		if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("splitCommandArgs(%q) = %q, want %q", cmd, args, want)
		}
	})
}
