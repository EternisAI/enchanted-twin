// Owner: slimane@eternis.ai
package twin_network

import (
	"testing"
)

func TestKeyGeneration(t *testing.T) {
	priv, err := GenerateRandomPrivateKey()
	if err != nil {
		t.Fatalf("GenerateRandomPrivateKey returned error: %v", err)
	}
	if priv == nil {
		t.Fatalf("private key is nil")
	}

	pub := DerivePublicKey(priv)
	if pub == nil {
		t.Fatalf("derived public key is nil")
	}
}

func TestAgentStoreAndRetrieve(t *testing.T) {
	agent, err := NewRandomAgentPubKey()
	if err != nil {
		t.Fatalf("NewRandomAgentPubKey error: %v", err)
	}
	if agent.PublicKey == nil {
		t.Fatalf("agent public key is nil")
	}

	StoreAgentKey(agent)

	retrieved, ok := GetAgentKey()
	if !ok {
		t.Fatalf("agent not found in store")
	}
	if retrieved != agent {
		t.Fatalf("retrieved agent does not match original")
	}
}

func TestSignAndVerify(t *testing.T) {
	agent, err := NewRandomAgentPubKey()
	if err != nil {
		t.Fatalf("NewRandomAgentPubKey error: %v", err)
	}

	msg := "hello world"
	sig, err := agent.SignMessage(msg)
	if err != nil {
		t.Fatalf("SignMessage error: %v", err)
	}
	if len(sig) != 65 {
		t.Fatalf("expected 65-byte signature, got %d", len(sig))
	}

	pubRecovered, err := RecoverPubKey(msg, []byte(sig))
	if err != nil {
		t.Fatalf("RecoverPubKey error: %v", err)
	}
	if pubRecovered == nil {
		t.Fatalf("recovered public key is nil")
	}

	if !VerifyMessageSignature(msg, sig, pubRecovered) {
		t.Fatalf("signature verification failed")
	}

	// verification should fail with a different message
	if VerifyMessageSignature("different message", sig, pubRecovered) {
		t.Fatalf("verification succeeded with wrong message; expected failure")
	}
}
