package security

import "testing"

func TestPBKDF2Hasher_HashAndCompare(t *testing.T) {
	hasher, err := NewPBKDF2Hasher(1000)
	if err != nil {
		t.Fatalf("NewPBKDF2Hasher() error: %v", err)
	}

	hash, err := hasher.Hash("Password123!")
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}

	ok, err := hasher.Compare(hash, "Password123!")
	if err != nil {
		t.Fatalf("Compare() error: %v", err)
	}
	if !ok {
		t.Fatalf("expected password to match")
	}

	ok, err = hasher.Compare(hash, "wrong")
	if err != nil {
		t.Fatalf("Compare() error: %v", err)
	}
	if ok {
		t.Fatalf("expected password mismatch")
	}
}

func TestPBKDF2Hasher_CompareInvalidFormat(t *testing.T) {
	hasher, _ := NewPBKDF2Hasher(1000)

	if _, err := hasher.Compare("bad-format", "Password123!"); err == nil {
		t.Fatalf("expected invalid format error")
	}
}

