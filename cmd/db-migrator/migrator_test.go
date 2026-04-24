package main

import "testing"

func TestCompatibleAppliedChecksum_AllowsKnownLegacyTransactionChecksum(t *testing.T) {
	reason, ok := compatibleAppliedChecksum(
		"transactions_db",
		"001_init.sql",
		"a35fb7e9443fef2ec1f22283682aed8860ef1b6dd29338fc3cd643e2fe1c6657",
	)
	if !ok {
		t.Fatalf("expected legacy checksum to be compatible")
	}
	if reason == "" {
		t.Fatalf("expected compatibility reason")
	}
}

func TestCompatibleAppliedChecksum_RejectsUnknownChecksum(t *testing.T) {
	_, ok := compatibleAppliedChecksum("transactions_db", "001_init.sql", "unknown")
	if ok {
		t.Fatalf("expected unknown checksum to be rejected")
	}
}
