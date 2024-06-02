package cryptest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

const (
	rsaBits         = 4096
	keyLifetimeSecs = uint32(86400 * 365)
)

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

	e := &openpgp.Entity{
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
	return e
}

// GenerateTestKey generates an openpgp.Key suitable for testing.
func GenerateTestKey(t *testing.T) openpgp.Key {
	now := time.Now()
	key, err := rsa.GenerateKey(rand.Reader, rsaBits)
	require.NoError(t, err)

	privKey := packet.NewRSAPrivateKey(now, key)
	pubKey := packet.NewRSAPublicKey(now, &key.PublicKey)
	return openpgp.Key{
		PrivateKey: privKey,
		PublicKey:  pubKey,
		Entity:     createEntityFromKeys(pubKey, privKey),
	}
}
