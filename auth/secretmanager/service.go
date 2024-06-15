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
package secretmanager

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/unstablebuild/blue/iterator"
	"github.com/ernestrc/go-multierror"
	giterator "google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Service abstracts the ability to securely manage secrets on
// a remote service provider.
type Service struct {
	projectID string
	parent    string
	client    *secretmanager.Client
}

// NewService allocates storage for a new Service and initializes it.
func NewService(projectID, credsFile string) (*Service, error) {
	// in a GC runtime, the SDK knows how to fetch credentials
	// for local development, we need to pass a file manually
	if credsFile != "" {
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credsFile)
	}

	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager new: %v", err)
	}

	// https://cloud.google.com/secret-manager/docs/reference/libraries
	parent := fmt.Sprintf("projects/%s", projectID)
	return &Service{
		projectID: projectID,
		client:    client,
		parent:    parent,
	}, nil
}

// CreateSecret creates a new secret with the given ID and payload.
func (s *Service) CreateSecret(ctx context.Context, ID string, metadata map[string]string) error {
	createSecretReq := &secretmanagerpb.CreateSecretRequest{
		Parent:   s.parent,
		SecretId: ID,
		Secret: &secretmanagerpb.Secret{
			Annotations: metadata,
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}
	_, err := s.client.CreateSecret(ctx, createSecretReq)
	if err != nil {
		return fmt.Errorf("client create secret: %v", err)
	}
	return nil
}

// GetSecret gets the secret with ID. This method does not access the secret's secret data.
func (s *Service) GetSecret(ctx context.Context, ID string) (Secret, error) {
	req := &secretmanagerpb.GetSecretRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s", s.projectID, ID),
	}

	resp, err := s.client.GetSecret(ctx, req)
	if err != nil {
		return Secret{}, fmt.Errorf("client get secret: %v", err)
	}

	return Secret{
		ID:          parseLastPathSegment(resp.Name),
		Annotations: resp.Annotations,
		CreatedAt:   protoTimeToStd(resp.CreateTime),
	}, nil
}

// RotateSecret disables any enabled versions of the given secret with ID
// after the given expireAfter and adds a new version which stores newData.
func (s *Service) RotateSecret(
	ctx context.Context, ID string, payload []byte,
	disablePreviousVersions bool,
) (int, error) {
	var versionsToDisable []string
	if disablePreviousVersions {
		// list all secret versions that are enabled
		listReq := secretmanagerpb.ListSecretVersionsRequest{
			Parent: fmt.Sprintf("projects/%s/secrets/%s", s.projectID, ID),
			Filter: "state:ENABLED",
		}

		it := s.client.ListSecretVersions(ctx, &listReq)
		for {
			next, err := it.Next()
			if err == giterator.Done {
				break
			}
			if err != nil {
				return 0, fmt.Errorf("iterate secret versions: %v", err)
			}
			versionsToDisable = append(versionsToDisable, next.Name)
		}
	}

	// add one more secret version with the new payload
	addReq := secretmanagerpb.AddSecretVersionRequest{
		Parent: fmt.Sprintf("projects/%s/secrets/%s", s.projectID, ID),
		Payload: &secretmanagerpb.SecretPayload{
			Data: payload,
		},
	}

	if _, err := s.client.AddSecretVersion(ctx, &addReq); err != nil {
		// no need to rollback anything here
		return 0, fmt.Errorf("client add secret version: %v", err)
	}

	// disable all previously enabled secret versions
	var ret error
	for _, version := range versionsToDisable {
		disableReq := secretmanagerpb.DisableSecretVersionRequest{
			Name: version,
		}
		if _, err := s.client.DisableSecretVersion(ctx, &disableReq); err != nil {
			err = fmt.Errorf("failed to disable previous secret version: %v", err)
			ret = multierror.Append(ret, err)
		}
	}
	return len(versionsToDisable), ret
}

// DisableSecretVersion disables the secret defined by ID's version.
func (s *Service) DisableSecretVersion(ctx context.Context, ID, version string) error {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", s.projectID, ID, version)
	disableReq := secretmanagerpb.DisableSecretVersionRequest{
		Name: name,
	}
	_, err := s.client.DisableSecretVersion(ctx, &disableReq)
	return err
}

// AccessSecretLatest gets the latest version of the secret with ID.
func (s *Service) AccessSecretLatest(ctx context.Context, ID string) (SecretVersion, error) {
	// https://cloud.google.com/secret-manager/docs/reference/rpc/google.cloud.secretmanager.v1#accesssecretversionrequest
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", s.projectID, ID)
	accessRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	accessRes, err := s.client.AccessSecretVersion(ctx, accessRequest)
	if err != nil {
		return SecretVersion{}, fmt.Errorf("client access secret: %v", err)
	}

	getRequest := &secretmanagerpb.GetSecretVersionRequest{
		Name: name,
	}

	getRes, err := s.client.GetSecretVersion(ctx, getRequest)
	if err != nil {
		return SecretVersion{}, fmt.Errorf("client get secret: %v", err)
	}

	version := parseLastPathSegment(accessRes.Name)
	return SecretVersion{
		ID:        ID,
		Version:   version,
		State:     protoStateToLib(getRes.State),
		Payload:   accessRes.Payload.Data,
		CreatedAt: protoTimeToStd(getRes.CreateTime),
	}, nil
}

// AccessSecretVersions gets all the enabled versions of the
// secret with ID.
func (s *Service) AccessSecretVersions(ctx context.Context, ID string) (
	[]SecretVersion, error,
) {
	req := &secretmanagerpb.ListSecretVersionsRequest{
		Parent: fmt.Sprintf("projects/%s/secrets/%s", s.projectID, ID),
		Filter: "state:ENABLED",
	}

	it := s.client.ListSecretVersions(ctx, req)

	var ret []SecretVersion
	var retErr error
	var mu sync.Mutex
	var wg sync.WaitGroup
	for {
		next, err := it.Next()
		if err == giterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterator error: %v", err)
		}
		wg.Add(1)
		go func(next *secretmanagerpb.SecretVersion) {
			defer wg.Done()
			req := secretmanagerpb.AccessSecretVersionRequest{Name: next.Name}
			resp, err := s.client.AccessSecretVersion(ctx, &req)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				retErr = multierror.Append(retErr, err)
				return
			}

			payload := SecretVersion{
				ID:        ID,
				Version:   parseLastPathSegment(next.Name),
				State:     protoStateToLib(next.State),
				CreatedAt: protoTimeToStd(next.CreateTime),
				Payload:   resp.Payload.Data}
			ret = append(ret, payload)
		}(next)
	}

	wg.Wait()
	return ret, retErr
}

// ListSecrets gets all the secret versions based on the given filters.
// See https://cloud.google.com/secret-manager/docs/filtering for filter format.
func (s *Service) ListSecrets(
	ctx context.Context, filter string,
) (iterator.Iterator[Secret], error) {
	req := secretmanagerpb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%s", s.projectID),
		Filter: filter,
	}

	iterator := s.client.ListSecrets(ctx, &req)
	return &versionsIterator{iter: iterator}, nil
}

// Close frees all resources associated with this Service.
func (s *Service) Close() error {
	return s.client.Close()
}

type versionsIterator struct {
	iter *secretmanager.SecretIterator
	err  error
}

func (i *versionsIterator) Next() (Secret, bool) {
	if i.err != nil {
		return Secret{}, false
	}
	next, err := i.iter.Next()
	if err == giterator.Done {
		return Secret{}, false
	}
	if err != nil {
		i.err = err
		return Secret{}, false
	}
	return Secret{
		ID:          parseLastPathSegment(next.Name),
		Annotations: next.Annotations,
		CreatedAt:   protoTimeToStd(next.CreateTime),
	}, true
}

func (i *versionsIterator) Err() error {
	return i.err
}

func protoTimeToStd(ts *timestamppb.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
}

func parseLastPathSegment(version string) string {
	split := strings.Split(version, "/")
	// best effort
	if len(split) <= 1 {
		return version
	}
	return split[len(split)-1]
}

func protoStateToLib(state secretmanagerpb.SecretVersion_State) State {
	switch state {
	case secretmanagerpb.SecretVersion_ENABLED:
		return StateEnabled
	case secretmanagerpb.SecretVersion_DISABLED:
		return StateDisabled
	case secretmanagerpb.SecretVersion_DESTROYED:
		return StateDestroyed
	default:
		// case secretmanagerpb.SecretVersion_STATE_UNSPECIFIED:
		return StateUnknown
	}
}
