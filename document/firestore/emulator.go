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
		_ = os.Setenv("FIRESTORE_EMULATOR_HOST", addr)
	}

	logging.LogResult(err, start, traceID, callType, field)
	return
}

func waitForEmulator(addr string) (err error) {
	const tries = 20
	const backoff = 500 * time.Millisecond
	var conn net.Conn
	for range tries {
		conn, err = net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(backoff)
	}
	return
}
