package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestECDHSharedVector pins X25519 against an independent BoringSSL vector so
// the Android implementation has a fixed cross-check target.
func TestECDHSharedVector(t *testing.T) {
	in := mustHex(t, "668fb9f76ad971c81ac900071a1560bce2ca00cac7e67af99348913761434014")
	base := mustHex(t, "db5f32b7f841e7a1a00968effded12735fc47a3eb13b579aacadeae80939a7dd")
	want := mustHex(t, "090d85e599ea8e2beeb61304d37be10ec5c905f9927d32f42a9a0afb3e0b4074")
	got, err := ECDHShared(in, base)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x want %x", got, want)
	}
}

// TestXChaCha20Vector pins the AEAD against draft-irtf-cfrg-xchacha A.3.1.
func TestXChaCha20Vector(t *testing.T) {
	key := mustHex(t, "808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9f")
	nonce := mustHex(t, "404142434445464748494a4b4c4d4e4f5051525354555657")
	aad := mustHex(t, "50515253c0c1c2c3c4c5c6c7")
	pt := mustHex(t, "4c616469657320616e642047656e746c656d656e206f662074686520636c617373206f66202739393a204966204920636f756c64206f6666657220796f75206f6e6c79206f6e652074697020666f7220746865206675747572652c2073756e73637265656e20776f756c642062652069742e")
	want := "bd6d179d3e83d43b9576579493c0e939572a1700252bfaccbed2902c21396cbb731c7f1b0b4aa6440bf3a82f4eda7e39ae64c6708c54c216cb96b72e1213b4522f8c9ba40db5d945b11b69b982c1bb9e3f3fac2bc369488f76b2383565d3fff921f9664c97637da9768812f615c68b13b52ec0875924c1c7987947deafd8780acf49"
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(aead.Seal(nil, nonce, pt, aad)); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestECDHSealRoundtrip(t *testing.T) {
	for _, scope := range []ECDHScope{ECDHScopeCredential, ECDHScopeUpdate} {
		cPriv, cPub, err := GenerateECDHKeypair()
		if err != nil {
			t.Fatal(err)
		}
		sPriv, _, err := GenerateECDHKeypair()
		if err != nil {
			t.Fatal(err)
		}
		pt := []byte(`{"baseUrl":"https://dav.example.com","username":"u"}`)
		sPub, nonce, ct, err := SealForClient(sPriv, cPub, pt, scope)
		if err != nil {
			t.Fatal(err)
		}
		if len(sPub) != curve25519.PointSize || len(nonce) != chacha20poly1305.NonceSizeX {
			t.Fatal("bad envelope sizes")
		}
		got, err := OpenFromServer(cPriv, sPub, nonce, ct, scope)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, pt) {
			t.Fatalf("scope %s: got %q want %q", scope, got, pt)
		}
		// Fresh seals must differ (random nonce).
		_, _, ct2, err := SealForClient(sPriv, cPub, pt, scope)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(ct, ct2) {
			t.Fatal("seals are deterministic")
		}
	}
}

func TestECDHScopeIsolation(t *testing.T) {
	cPriv, cPub, _ := GenerateECDHKeypair()
	sPriv, _, _ := GenerateECDHKeypair()
	sPub, nonce, ct, err := SealForClient(sPriv, cPub, []byte("secret"), ECDHScopeCredential)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFromServer(cPriv, sPub, nonce, ct, ECDHScopeUpdate); err == nil {
		t.Fatal("cross-scope open succeeded")
	}
}

func TestECDHRejects(t *testing.T) {
	cPriv, cPub, _ := GenerateECDHKeypair()
	sPriv, _, _ := GenerateECDHKeypair()
	sPub, nonce, ct, err := SealForClient(sPriv, cPub, []byte("secret"), ECDHScopeCredential)
	if err != nil {
		t.Fatal(err)
	}
	// Tampered ciphertext.
	bad := bytes.Clone(ct)
	bad[0] ^= 0x80
	if _, err := OpenFromServer(cPriv, sPub, nonce, bad, ECDHScopeCredential); err == nil {
		t.Fatal("tampered open succeeded")
	}
	// Wrong recipient key.
	oPriv, _, _ := GenerateECDHKeypair()
	if _, err := OpenFromServer(oPriv, sPub, nonce, ct, ECDHScopeCredential); err == nil {
		t.Fatal("wrong-key open succeeded")
	}
	// Weak / malformed peers.
	if _, _, _, err := SealForClient(sPriv, make([]byte, 32), []byte("x"), ECDHScopeCredential); err == nil {
		t.Fatal("all-zero peer accepted")
	}
	if _, _, _, err := SealForClient(sPriv, []byte("short"), []byte("x"), ECDHScopeCredential); err == nil {
		t.Fatal("short peer accepted")
	}
	if _, err := OpenFromServer(cPriv, sPub, []byte("short-nonce"), ct, ECDHScopeCredential); err == nil {
		t.Fatal("short nonce accepted")
	}
}
