package crypto

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

// Key identifies a specific public key in an Entity. This is either the
// Entity's primary key or a subkey. See openpgp.Key for more details.
type Key openpgp.Key

// PrimaryIdentity returns the Identity marked as primary or the first identity
// if none are so marked.
func (k Key) PrimaryIdentity() *openpgp.Identity {
	var firstIdentity *openpgp.Identity
	for _, ident := range k.Entity.Identities {
		if firstIdentity == nil {
			firstIdentity = ident
		}
		if ident.SelfSignature.IsPrimaryId != nil && *ident.SelfSignature.IsPrimaryId {
			return ident
		}
	}
	return firstIdentity
}

func decodeSignature(in io.Reader) (*packet.Signature, error) {
	block, err := armor.Decode(in)
	if err != nil {
		return nil, fmt.Errorf("Error decoding OpenPGP Armor: %s", err)
	}

	if block.Type != openpgp.SignatureType {
		return nil, errors.New("Error decoding signature: Invalid signature file")
	}

	reader := packet.NewReader(block.Body)
	pkt, err := reader.Next()
	if err != nil {
		return nil, fmt.Errorf("Error reading signature: %s", err)
	}

	sig, ok := pkt.(*packet.Signature)
	if !ok {
		return nil, errors.New("Error parsing signature: Invalid signature")
	}
	return sig, nil
}

// FindKeysInKeyRing finds the key with ID in keyringFile. If passphrase is not an empty string
// and a private key that needs decrypting is founds, the passphrase will be used to decrypt it.
// It returns an error if no keys are found in keyring.
func FindKeysInArmoredKeyRing(keyringFile, ID, passphrase string) (e []Key, err error) {
	keyringFileBuffer, err := os.Open(keyringFile)
	if err != nil {
		err = fmt.Errorf("Error opening armored keyring: %s", err)
		return nil, err
	}
	defer keyringFileBuffer.Close()
	entityList, err := openpgp.ReadArmoredKeyRing(keyringFileBuffer)
	if err != nil {
		err = fmt.Errorf("Error reading armored keyring: %s", err)
		return
	}

	uintID, err := strconv.ParseUint(fmt.Sprintf("0x%s", ID), 0, 64)
	if err != nil {
		err = fmt.Errorf("Error parsing key ID: %s", err)
		return
	}

	keys := entityList.KeysById(uintID)
	if len(keys) == 0 {
		err = fmt.Errorf("Error key with ID %s not found in keyring %s", ID, keyringFile)
		return
	}

	// decrypt private key if available.
	for _, k := range keys {
		e = append(e, Key(k))

		if k.PrivateKey == nil {
			continue
		}
		if k.PrivateKey.Encrypted && passphrase != "" {
			err = k.PrivateKey.Decrypt([]byte(passphrase))
			if err != nil {
				err = fmt.Errorf("Error decrypting key with ID '%s' with given passphrase: %s", ID, err)
				return
			}
		}
	}

	return e, nil
}

// ArmoredSign signs message with the private key and writes an armored signature to out.
// Note that PrivateKey must have been decrypted if it is encrypted. See FindPrivateKey
// for more details.
func ArmoredSign(in io.Reader, out io.Writer, key Key) error {
	err := openpgp.ArmoredDetachSign(out, (openpgp.Key)(key).Entity, in, nil)
	if err != nil {
		return fmt.Errorf("Error signing input: %s", err)
	}

	return nil
}

// Verify returns nil if sig is a valid signature of in
// and made by public key in pubKeyFile.
func Verify(in, sig io.Reader, key Key) error {
	signature, err := decodeSignature(sig)
	if err != nil {
		return err
	}

	h := signature.Hash.New()
	_, _ = io.Copy(h, in)

	err = key.Entity.PrimaryKey.VerifySignature(h, signature)
	if err != nil {
		return fmt.Errorf("Error signing input: %s", err)
	}

	return nil
}
