package graph

import (
	"fmt"
	"strings"
)

// Allowed HTTP methods when writes are disabled (default).
var readOnlyMethods = map[string]bool{
	"GET":     true,
	"HEAD":    true,
	"OPTIONS": true,
}

// Methods that are always blocked regardless of --allow-writes.
var blockedMethods = map[string]bool{
	"DELETE": true,
}

// CheckSafety enforces the write protection policy.
//
// Default behavior (allowWrites=false): only GET, HEAD, OPTIONS are allowed.
// With --allow-writes: POST, PUT, PATCH are additionally allowed.
// DELETE is always rejected.
func CheckSafety(method string, allowWrites bool) error {
	method = strings.ToUpper(method)

	// DELETE is always blocked
	if blockedMethods[method] {
		return fmt.Errorf("DELETE requests are blocked for safety. This tool does not support DELETE operations")
	}

	// If writes are allowed, any non-blocked method is fine
	if allowWrites {
		return nil
	}

	// Default: read-only
	if !readOnlyMethods[method] {
		return fmt.Errorf(
			"write operation %q blocked: use --allow-writes flag to enable POST/PUT/PATCH requests. "+
				"The agent should confirm with the user before making write operations",
			method,
		)
	}

	return nil
}
