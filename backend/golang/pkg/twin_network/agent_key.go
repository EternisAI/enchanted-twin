// Owner: slimane@eternis.ai

package twin_network

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
)

type AgentKey struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

// GenerateRandomPrivateKey creates a brand-new secp256k1 private key.
func GenerateRandomPrivateKey() (*ecdsa.PrivateKey, error) {
	return crypto.GenerateKey()
}

// DerivePublicKey derives the public key that corresponds to a private key.
func DerivePublicKey(priv *ecdsa.PrivateKey) *ecdsa.PublicKey {
	if priv == nil {
		return nil
	}
	return &priv.PublicKey
}

// GenerateRandomPublicKey generates a random public key by internally.
func GenerateRandomPublicKey() (*ecdsa.PublicKey, error) {
	priv, err := GenerateRandomPrivateKey()
	if err != nil {
		return nil, err
	}
	return DerivePublicKey(priv), nil
}

// NewRandomAgentPubKey returns a fresh AgentPubKey with a unique key-pair.
func NewRandomAgentPubKey() (*AgentKey, error) {
	priv, err := GenerateRandomPrivateKey()
	if err != nil {
		return nil, err
	}
	return &AgentKey{
		PrivateKey: priv,
		PublicKey:  &priv.PublicKey,
	}, nil
}

// For the current design we assume each running Twin agent owns exactly one
// key-pair. We therefore keep a single instance in memory. In production you
// might load this from an encrypted keystore file or a remote KMS.

var store = struct {
	sync.RWMutex
	agent *AgentKey
}{}

// StoreAgentKey replaces whatever key is currently cached with the given one.
func StoreAgentKey(agent *AgentKey) {
	if agent == nil || agent.PublicKey == nil {
		return
	}
	store.Lock()
	store.agent = agent
	store.Unlock()
}

// GetAgentKey returns the current in-memory AgentKey (if any) together with a
// boolean indicating whether a key has been stored.
func GetAgentKey() (*AgentKey, bool) {
	store.RLock()
	defer store.RUnlock()
	if store.agent == nil {
		return nil, false
	}
	return store.agent, true
}

// PubKeyHex returns the lowercase hex-encoded (uncompressed) public key for
// convenience when you only need the identifier.
func (a *AgentKey) PubKeyHex() string {
	if a == nil || a.PublicKey == nil {
		return ""
	}
	return strings.ToLower(hex.EncodeToString(crypto.FromECDSAPub(a.PublicKey)))
}

// SignMessage signs an arbitrary ASCII/UTF-8 string using the agent's private
// key. The signature is 65 bytes long: {R || S || V} where V is the recovery
// identifier (0 or 1) as produced by go-ethereum/crypto.Sign.
func (a *AgentKey) SignMessage(msg string) (string, error) {
	if a == nil || a.PrivateKey == nil {
		return "", errors.New("agent has no private key")
	}
	hash := crypto.Keccak256([]byte(msg))
	signature, err := crypto.Sign(hash, a.PrivateKey)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(signature), nil
}

// RecoverPubKey derives the public key that produced the signature for the
// given message. It returns an error if the signature is malformed or does not
// correspond to a valid key.
func RecoverPubKey(msg string, signature []byte) (*ecdsa.PublicKey, error) {
	if len(signature) != 65 {
		return nil, errors.New("invalid signature length")
	}
	hash := crypto.Keccak256([]byte(msg))
	return crypto.SigToPub(hash, signature)
}

// VerifyMessageSignature verifies that `signature` is a valid signature of
// `msg` by the given public key.
func VerifyMessageSignature(msg string, signature string, pubKey *ecdsa.PublicKey) bool {
	if len(signature) != 65 || pubKey == nil {
		return false
	}
	hash := crypto.Keccak256([]byte(msg))
	sigNoV, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return crypto.VerifySignature(crypto.FromECDSAPub(pubKey), hash, sigNoV)
}
