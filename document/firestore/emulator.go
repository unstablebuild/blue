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
package firestore

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
)

// RunFirestoreEmulator runs firestore emulator as a subprocess.
//
// It sets FIRESTORE_EMULATOR_HOST so calls to firestore.NewClient
// know that emulator is to be used instead of real Firestore.
//
// Gloud SDK and the emulator need to be installed first.
func RunFirestoreEmulator() (teardown func() error, err error) {
	const addr = "0.0.0.0:8045"
	const callType = "StartFirestoreEmulator"

	cmd := exec.Command("gcloud", "beta", "emulators", "firestore", "start", fmt.Sprintf("--host-port=%s", addr))
	err = cmd.Start()
	if err != nil {
		return
	}

	traceID := trace.New()
	field := logging.Field{Key: "address", Value: addr}
	start := logging.LogAttempt(traceID, callType, field)

	teardown = cmd.Process.Kill
	err = waitForEmulator(addr)
	if err != nil {
		_ = teardown()
	} else {
		os.Setenv("FIRESTORE_EMULATOR_HOST", addr)
	}

	logging.LogResult(err, start, traceID, callType, field)
	return
}

func waitForEmulator(addr string) (err error) {
	const tries = 20
	const backoff = 500 * time.Millisecond
	var conn net.Conn
	for i := 0; i < tries; i++ {
		conn, err = net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(backoff)
	}
	return
}
