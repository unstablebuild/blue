package crypto

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

const (
	rsaBits         = 4096
	keyLifetimeSecs = uint32(86400 * 365)
)

type key interface {
	Serialize(io.Writer) error
}

func createEntityFromKeys(pubKey *packet.PublicKey, privKey *packet.PrivateKey) *openpgp.Entity {
	config := packet.Config{
		DefaultHash:            crypto.SHA256,
		DefaultCipher:          packet.CipherAES256,
		DefaultCompressionAlgo: packet.CompressionZLIB,
		CompressionConfig: &packet.CompressionConfig{
			Level: 9,
		},
		RSABits: rsaBits,
	}
	currentTime := config.Now()
	uid := packet.NewUserId("", "", "")

	e := openpgp.Entity{
		PrimaryKey: pubKey,
		PrivateKey: privKey,
		Identities: make(map[string]*openpgp.Identity),
	}
	isPrimaryID := false

	e.Identities[uid.Id] = &openpgp.Identity{
		Name:   uid.Name,
		UserId: uid,
		SelfSignature: &packet.Signature{
			CreationTime: currentTime,
			SigType:      packet.SigTypePositiveCert,
			PubKeyAlgo:   packet.PubKeyAlgoRSA,
			Hash:         config.Hash(),
			IsPrimaryId:  &isPrimaryID,
			FlagsValid:   true,
			FlagSign:     true,
			FlagCertify:  true,
			IssuerKeyId:  &e.PrimaryKey.KeyId,
		},
	}

	keyLifetimeSecs := keyLifetimeSecs
	e.Subkeys = make([]openpgp.Subkey, 1)
	e.Subkeys[0] = openpgp.Subkey{
		PublicKey:  pubKey,
		PrivateKey: privKey,
		Sig: &packet.Signature{
			CreationTime:              currentTime,
			SigType:                   packet.SigTypeSubkeyBinding,
			PubKeyAlgo:                packet.PubKeyAlgoRSA,
			Hash:                      config.Hash(),
			PreferredHash:             []uint8{8}, // SHA-256
			FlagsValid:                true,
			FlagEncryptStorage:        true,
			FlagEncryptCommunications: true,
			IssuerKeyId:               &e.PrimaryKey.KeyId,
			KeyLifetimeSecs:           &keyLifetimeSecs,
		},
	}
	return &e
}

func generateKeyPair(t *testing.T) (Key, Key) {
	now := time.Now()
	key, err := rsa.GenerateKey(rand.Reader, rsaBits)
	require.NoError(t, err)

	privKey := packet.NewRSAPrivateKey(now, key)
	pubKey := packet.NewRSAPublicKey(now, &key.PublicKey)
	k := Key(openpgp.Key{
		PrivateKey: privKey,
		PublicKey:  pubKey,
		Entity:     createEntityFromKeys(pubKey, privKey),
	})

	return k, k
}

func TestSignVerify(t *testing.T) {
	t.Run("verifies signature correctly", func(t *testing.T) {
		pubKey, privKey := generateKeyPair(t)

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, privKey)
		require.NoError(t, err)

		err = Verify(strings.NewReader(str), &out, pubKey)
		assert.NoError(t, err)
	})

	t.Run("returns error if data has been tampered with", func(t *testing.T) {
		pubKey, privKey := generateKeyPair(t)

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, privKey)
		require.NoError(t, err)

		str = "bluectl cryp\x00"
		err = Verify(strings.NewReader(str), &out, pubKey)
		assert.Error(t, err)
	})

	t.Run("returns error if a different key pair has been used to sign data", func(t *testing.T) {
		_, privKey := generateKeyPair(t)
		pubKeyFile2, _ := generateKeyPair(t)

		str := "bluectl crypo"

		var out bytes.Buffer
		err := ArmoredSign(strings.NewReader(str), &out, privKey)
		require.NoError(t, err)

		err = Verify(strings.NewReader(str), &out, pubKeyFile2)
		assert.Error(t, err)
	})
}
