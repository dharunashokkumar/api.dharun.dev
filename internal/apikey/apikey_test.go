package apikey

import "testing"

func TestKeys(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		key, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if Check(key) != nil || !Looks(key) || len(key) != Length || seen[key] {
			t.Fatalf("bad key %q", key)
		}
		seen[key] = true
		// One changed character is refused without a lookup.
		changed := []byte(key)
		if changed[20] == 'a' {
			changed[20] = 'b'
		} else {
			changed[20] = 'a'
		}
		if Check(string(changed)) == nil {
			t.Errorf("a mistyped key %q passed the checksum", changed)
		}
	}
	for _, bad := range []string{"", "india_live_", "sk_live_abc", Prefix + "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"} {
		if Check(bad) == nil {
			t.Errorf("Check(%q) passed", bad)
		}
	}
	if Hash("x") == Hash("y") || len(Hash("x")) != 64 {
		t.Error("hash")
	}
	if Hint(Prefix+"abcdefghijklmnopqrstuvwxyz0123456789ABCDEF") != Prefix+"…CDEF" {
		t.Error("hint")
	}
}
