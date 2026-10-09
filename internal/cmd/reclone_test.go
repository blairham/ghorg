// SPDX-FileCopyrightText: 2018 gabrie30 and the gabrie30/ghorg contributors
// SPDX-FileCopyrightText: 2025 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cmd

import "testing"

func Test_sanitizeCmd(t *testing.T) {
	type args struct {
		cmd string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "shorthand with space",
			args: args{cmd: "ghorg clone foo -t bGVhdmUgYSBjb21tZW50IG9uIGlzc3VlIDY2"},
			want: "ghorg clone foo -t XXXXXXX",
		},
		{
			name: "shorthand with equals",
			args: args{cmd: "ghorg clone foo -t=bGVhdmUgYSBjb21tZW50IG9uIGlzc3VlIDY2"},
			want: "ghorg clone foo -t=XXXXXXX",
		},
		{
			name: "longhand with space",
			args: args{cmd: "ghorg clone foo --token bGVhdmUgYSBjb21tZW50IG9uIGlzc3VlIDY2"},
			want: "ghorg clone foo --token XXXXXXX",
		},
		{
			name: "longhand with equals",
			args: args{cmd: "ghorg clone foo --token=bGVhdmUgYSBjb21tZW50IG9uIGlzc3VlIDY2"},
			want: "ghorg clone foo --token=XXXXXXX",
		},
		{
			name: "shorthand with equals does not pick up other flags with t",
			args: args{cmd: "ghorg clone foo -t=bGVhdmUgYSBjb21tZW50IG9uIGlzc3VlIDY2 --topics=foo,bar"},
			want: "ghorg clone foo -t=XXXXXXX --topics=foo,bar",
		},
		{
			name: "shorthand with space does not pick up other flags with t",
			args: args{cmd: "ghorg clone foo -t bGVhdmUgYSBjb21tZW50IG9uIGlzc3VlIDY2 --topics=foo,bar"},
			want: "ghorg clone foo -t XXXXXXX --topics=foo,bar",
		},
		{
			name: "a -t= inside another flag's value does not hide the real token",
			args: args{cmd: "ghorg clone foo --match-regex=a-t=b --token=faketokenvalue"},
			want: "ghorg clone foo --match-regex=a-t=b --token=XXXXXXX",
		},
		{
			name: "every token is masked, and the arguments after them kept",
			args: args{cmd: "ghorg clone foo --token=one1one1 --skip-forks -t two2two2 --bitbucket-api-token=three333"},
			want: "ghorg clone foo --token=XXXXXXX --skip-forks -t XXXXXXX --bitbucket-api-token=XXXXXXX",
		},
		{
			name: "a quoted token is masked whole",
			args: args{cmd: "ghorg clone foo --token='a token with spaces' --skip-forks"},
			want: "ghorg clone foo --token=XXXXXXX --skip-forks",
		},
		{
			name: "flags that merely start with t are left alone",
			args: args{cmd: "ghorg clone foo --topics=t1 -tx"},
			want: "ghorg clone foo --topics=t1 -tx",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeCmd(tt.args.cmd); got != tt.want {
				t.Errorf("sanitizeCmd() = %v, want %v", got, tt.want)
			}
		})
	}
}
