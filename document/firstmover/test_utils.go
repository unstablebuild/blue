package firstmover

import "github.com/ernestrc/blue/document"

// TestIsLeader returns true if this instance of
// firstmover document.Service is the leader.
// This method panics if s is not an instance of
// firstmover document.Service. It should only be used
// in tests.
func TestIsLeader(s document.Service) bool {
	return s.(*service).isLeader()
}
