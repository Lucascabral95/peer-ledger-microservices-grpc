package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestTransactionInitialMigrationChecksum(t *testing.T) {
	const expectedChecksum = "ae4beabc08ce5c599ada165f1a625c53e9779d3b295bbd32fe81108f3053b272"

	contents, err := migrationFS.ReadFile("sql/transactions/001_init.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	normalizedContents := strings.ReplaceAll(string(contents), "\r\n", "\n")
	checksum := sha256.Sum256([]byte(normalizedContents))
	actualChecksum := hex.EncodeToString(checksum[:])

	if actualChecksum != expectedChecksum {
		t.Fatalf("transactions 001_init.sql checksum changed: expected %s, got %s", expectedChecksum, actualChecksum)
	}
}
