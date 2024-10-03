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

package firstmover

import (
	"time"

	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
)

// Config holds configuration for a firstmover document.Service
type Config struct {
	Marshaler docmarshal.Marshaler
	// TransientFailureRecoverTimeout is the timeout until a grpc
	// transient connection failure is considered unrecoverable..
	TransientFailureRecoverTimeout time.Duration
	// MethodRetryCadence is the method retry timeout until next attempt.
	MethodRetryCadence time.Duration
	// ReceiveRetryCadence is the receive retry cadence to accomodate
	// new leader/follower assigns. It should be much much longer than
	// MethodRetryCadence as receive is expected to block.
	ReceiveRetryCadence time.Duration
	// ConnectRetryCadence is the connect retry timeout until next attempt.
	ConnectRetryCadence time.Duration
	// TimeToCoup is the time for a follower to take the lead
	// if leader is unresponsive.
	TimeToCoup time.Duration
	// DialTimeout is net.Dial timeout
	DialTimeout time.Duration
	// CloseError can be optionally set to an error value that
	// the underlying document.Service passes when it's been
	// called Close and any other calls to its API will fail.
	// This error will be retried by followers until a new leader
	// is selected.
	CloseError error
	// MaxMessageSize determines the maximum message size
	// of published messages via Service.Publish.
	MaxMessageSize int
}

// DefaultConfig returns a sane Config.
func DefaultConfig() Config {
	return Config{
		Marshaler:                      docbson.Marshaler(),
		TransientFailureRecoverTimeout: 500 * time.Millisecond,
		MethodRetryCadence:             200 * time.Millisecond,
		ReceiveRetryCadence:            5 * time.Second,
		ConnectRetryCadence:            50 * time.Millisecond,
		TimeToCoup:                     1 * time.Second,
		DialTimeout:                    40 * time.Millisecond,
		MaxMessageSize:                 DefaultMaxMessageSize,
	}
}
