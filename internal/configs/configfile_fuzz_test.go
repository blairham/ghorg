// SPDX-FileCopyrightText: 2025 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package configs

import (
	"path/filepath"
	"strings"
	"testing"
)

// FuzzConfigValueRoundTrip checks what `ghorg config set` promises: a value
// written to a registered key reads back unchanged, and writing a second key
// leaves the first alone.
func FuzzConfigValueRoundTrip(f *testing.F) {
	f.Add(uint16(0), uint16(1), "github", "value")
	f.Add(uint16(3), uint16(7), "true", "25")
	f.Add(uint16(5), uint16(5), "", "~/ghorg")
	f.Add(uint16(9), uint16(2), "null", "- a\n- b")
	f.Add(uint16(11), uint16(4), "0x10", "a: b")

	f.Fuzz(func(t *testing.T, first, second uint16, firstValue, secondValue string) {
		keyA := AllKeys[int(first)%len(AllKeys)].DotNotation
		keyB := AllKeys[int(second)%len(AllKeys)].DotNotation
		path := filepath.Join(t.TempDir(), "conf.yaml")

		if err := WriteConfigValue(path, keyA, firstValue); err != nil {
			t.Skip() // a value YAML cannot encode is refused, not mangled
		}
		if keyA != keyB {
			if err := WriteConfigValue(path, keyB, secondValue); err != nil {
				t.Skip()
			}
		}

		got, ok, err := ReadConfigValue(path, keyA)
		if err != nil || !ok || got != firstValue {
			t.Fatalf("set %s=%q then %s=%q; get %s = %q, %v, %v; want %q",
				keyA, firstValue, keyB, secondValue, keyA, got, ok, err, firstValue)
		}
	})
}

// FuzzFormatConfigListMasksSecrets checks that `ghorg config --list` never
// prints a secret's value unless asked to.
func FuzzFormatConfigListMasksSecrets(f *testing.F) {
	f.Add(uint16(0), "ghp_0123456789abcdef")
	f.Add(uint16(1), "glpat-abcdefghij")

	var secrets []string
	for _, k := range AllKeys {
		if k.IsSecret {
			secrets = append(secrets, k.DotNotation)
		}
	}
	f.Fuzz(func(t *testing.T, pick uint16, secret string) {
		key := secrets[int(pick)%len(secrets)]
		path := filepath.Join(t.TempDir(), "conf.yaml")
		if err := WriteConfigValue(path, key, secret); err != nil {
			t.Skip()
		}
		values, err := ListConfigValues(path)
		if err != nil {
			t.Fatalf("ListConfigValues: %v", err)
		}
		out := FormatConfigList(values, false)
		// The key's own name, and the mask, are in the output by design.
		if len(secret) < 8 || strings.Contains(key+"=********\n", secret) {
			return
		}
		if strings.Contains(out, secret) {
			t.Fatalf("FormatConfigList printed secret %s=%q: %q", key, secret, out)
		}
	})
}
