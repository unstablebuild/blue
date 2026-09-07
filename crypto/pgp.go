// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crypto

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
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
		return nil, fmt.Errorf("decode OpenPGP Armor: %s", err)
	}

	if block.Type != openpgp.SignatureType {
		return nil, errors.New("invalid signature file")
	}

	reader := packet.NewReader(block.Body)
	pkt, err := reader.Next()
	if err != nil {
		return nil, fmt.Errorf("read signature: %s", err)
	}

	sig, ok := pkt.(*packet.Signature)
	if !ok {
		return nil, errors.New("interpret signature: invalid type")
	}
	return sig, nil
}

// FindKeysInKeyRing finds the key with ID in keyringFile. If passphrase is not an empty string
// and a private key that needs decrypting is founds, the passphrase will be used to decrypt it.
// It returns an error if no keys are found in keyring.
func FindKeysInArmoredKeyRing(keyringFile, ID, passphrase string) (e []Key, err error) {
	keyringFileBuffer, err := os.Open(keyringFile)
	if err != nil {
		err = fmt.Errorf("open armored keyring: %s", err)
		return nil, err
	}
	defer func() { _ = keyringFileBuffer.Close() }()
	entityList, err := openpgp.ReadArmoredKeyRing(keyringFileBuffer)
	if err != nil {
		err = fmt.Errorf("read armored keyring: %s", err)
		return
	}

	uintID, err := strconv.ParseUint(fmt.Sprintf("0x%s", ID), 0, 64)
	if err != nil {
		err = fmt.Errorf("parse key ID: %s", err)
		return
	}

	keys := entityList.KeysById(uintID)
	if len(keys) == 0 {
		err = fmt.Errorf("key with ID %s not found in keyring %s", ID, keyringFile)
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
				err = fmt.Errorf("decrypt key with ID '%s' with given passphrase: %s", ID, err)
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
		return fmt.Errorf("sign input: %s", err)
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
		return fmt.Errorf("sign input: %s", err)
	}

	return nil
}
