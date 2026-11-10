package executor

import "bytes"

// transientMarkers are stderr/stdout signatures of a network or gateway fault
// that a later attempt can succeed past. They are not semantic gh results
// (failed checks, missing objects, bad credentials).
var transientMarkers = [][]byte{
	[]byte("operation timed out"),
	[]byte("i/o timeout"),
	[]byte("Client.Timeout exceeded"),
	[]byte("connection reset by peer"),
	[]byte("connection refused"),
	[]byte("TLS handshake timeout"),
	[]byte("network is unreachable"),
	[]byte("no such host"),
	[]byte("temporary failure in name resolution"),
	[]byte("unexpected EOF"),
	[]byte("HTTP 502"),
	[]byte("HTTP 503"),
	[]byte("HTTP 504"),
}

// uncacheableMarkers are completed-read failures that must not be replayed.
// Auth and client errors are not retried by a watch loop; they are still
// uncacheable so the next caller observes a fresh failure.
var uncacheableMarkers = [][]byte{
	[]byte("HTTP 401"),
	[]byte("HTTP 403"),
	[]byte("HTTP 404"),
	[]byte("HTTP 500"),
	[]byte("Bad credentials"),
}

// IsTransientNetwork reports whether output describes a transport or gateway
// fault worth retrying. A zero-length buffer is not transient: gh pr checks
// exits 1 for failed checks with the listing on stdout and empty stderr.
func IsTransientNetwork(output []byte) bool {
	if len(output) == 0 {
		return false
	}
	for _, marker := range transientMarkers {
		if bytes.Contains(output, marker) {
			return true
		}
	}
	return false
}

// IsUncacheableFailure reports whether a non-zero gh result must not be stored.
// Transport faults, auth failures, and HTTP client/server errors are included.
// Semantic exits such as pending checks are not.
func IsUncacheableFailure(output []byte) bool {
	if IsTransientNetwork(output) {
		return true
	}
	for _, marker := range uncacheableMarkers {
		if bytes.Contains(output, marker) {
			return true
		}
	}
	return false
}
