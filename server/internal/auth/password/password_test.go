package password

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func testParams() Params {
	return Params{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}
}

func TestHashAndVerify(t *testing.T) {
	hasher, err := newHasher(testParams(), bytes.NewReader(bytes.Repeat([]byte{0x42}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "correct horse") {
		t.Fatal("password hash contains plaintext")
	}
	match, needsRehash, err := hasher.Verify(encoded, "correct horse battery staple")
	if err != nil || !match || needsRehash {
		t.Fatalf("verify current hash: match=%v needsRehash=%v err=%v", match, needsRehash, err)
	}
	match, _, err = hasher.Verify(encoded, "wrong")
	if err != nil || match {
		t.Fatalf("wrong password: match=%v err=%v", match, err)
	}
}

func TestVerifyReportsPolicyUpgrade(t *testing.T) {
	oldHasher, err := newHasher(testParams(), bytes.NewReader(bytes.Repeat([]byte{0x11}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := oldHasher.Hash("long enough password")
	if err != nil {
		t.Fatal(err)
	}
	newParams := testParams()
	newParams.Iterations = 2
	newHasher, err := newHasher(newParams, bytes.NewReader(bytes.Repeat([]byte{0x22}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	match, needsRehash, err := newHasher.Verify(encoded, "long enough password")
	if err != nil || !match || !needsRehash {
		t.Fatalf("upgrade detection: match=%v needsRehash=%v err=%v", match, needsRehash, err)
	}
}

func TestVerifyRejectsMalformedAndExcessiveHashes(t *testing.T) {
	hasher, err := New(testParams())
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"not-a-hash",
		"$argon2id$v=18$m=8192,t=1,p=1$YWJjZGVmZ2hpamtsbW5vcA$YWJjZGVmZ2hpamtsbW5vcA",
		"$argon2id$v=19$m=999999,t=1,p=1$YWJjZGVmZ2hpamtsbW5vcA$YWJjZGVmZ2hpamtsbW5vcA",
	}
	for _, encoded := range cases {
		_, _, err := hasher.Verify(encoded, "password")
		if !errors.Is(err, ErrInvalidHash) {
			t.Fatalf("Verify(%q) error = %v", encoded, err)
		}
	}
}
