package executor

import "bytes"

// trustMarkers are the stderr signatures of a TLS *trust-evaluation* failure — the
// platform refusing to build a chain to a trusted root — as opposed to an ordinary
// API error, a 4xx, or a plain network fault.
//
// These are matched on the child `gh` process's stderr. Go's macOS verifier surfaces
// Security.framework failures as a bare OSStatus, e.g.
//
//	Post "https://api.github.com/graphql": tls: failed to verify certificate:
//	x509: OSStatus -26276
//
// which is a trust-store/keychain fault, NOT an expired or invalid credential. The
// distinction matters operationally: the wrong reflex here is to rotate a token,
// which burns a working credential and does not touch the cause.
var trustMarkers = [][]byte{
	[]byte("tls: failed to verify certificate"),
	[]byte("x509: OSStatus"),
	[]byte("x509: certificate signed by unknown authority"),
}

// IsTrustFailure reports whether a gh invocation failed because TLS trust evaluation
// failed, rather than for any ordinary reason.
//
// Why the daemon cares: every `gh` on the machine is the ghx client, and the client
// deliberately does not fall back to running gh directly (that would risk silent
// double-execution of mutations). So one daemon holding a broken TLS context fails
// EVERY gh call on the host, identically, while a plain `curl` still succeeds — the
// signature of the 2026-08-17 incident. The daemon spawns children with
// authenv.Apply(os.Environ(), ...), so the proxy/CA/GODEBUG half of the environment
// is frozen at daemon birth and replayed forever; a daemon born into a bad TLS
// context can never recover on its own.
//
// A zero exit is never a trust failure: gh writes assorted diagnostics to stderr on
// success, and treating those as faults would reap a healthy daemon.
func IsTrustFailure(r *Result) bool {
	if r == nil || r.ExitCode == 0 {
		return false
	}
	for _, marker := range trustMarkers {
		if bytes.Contains(r.Stderr, marker) {
			return true
		}
	}
	return false
}
