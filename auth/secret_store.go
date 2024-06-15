// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package auth

import (
	"context"
	"encoding/base64"
	"errors"
)

// EncodeSecretID encodes the given ID to be compatible with
// SecretStore implementations.
func EncodeSecretID(id string) string {
	return base64.StdEncoding.WithPadding(base64.NoPadding).
		EncodeToString([]byte(id))
}

// SecretStore abstracts the ability to retrieve secret metadata and access secret data.
type SecretStore interface {
	GetSecretMetadata(ctx context.Context, ID string) (map[string]string, error)
	AccessSecret(ctx context.Context, ID string) ([]byte, error)
}

// MapSecretStore returns a SecretStore of secrets with metadata.
// The argument metadata is expected to be pairs of key values and it is
// returned to all secrets.
func MapSecretStore(data map[string][]byte, metadata ...string) SecretStore {
	metadataMap := make(map[string]string)
	var prevKv string
	for i, kv := range metadata {
		if i%2 == 0 {
			prevKv = kv
			metadataMap[kv] = ""
		} else {
			metadataMap[prevKv] = kv
		}
	}
	return mapSecretStore{data: data, metadata: metadataMap}
}

type mapSecretStore struct {
	data     map[string][]byte
	metadata map[string]string
}

func (m mapSecretStore) GetSecretMetadata(ctx context.Context, ID string) (map[string]string, error) {
	ret := make(map[string]string, len(m.metadata))
	for k, v := range m.metadata {
		ret[k] = v
	}
	return ret, nil
}

func (m mapSecretStore) AccessSecret(ctx context.Context, ID string) ([]byte, error) {
	data, ok := m.data[ID]
	if !ok {
		return nil, errors.New("secret not found")
	}
	return data, nil
}
