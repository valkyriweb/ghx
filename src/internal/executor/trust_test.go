package executor

import "testing"

func TestIsTrustFailure(t *testing.T) {
	tests := []struct {
		name string
		res  *Result
		want bool
	}{
		{
			name: "nil result is not a trust failure",
			res:  nil,
			want: false,
		},
		{
			name: "success is never a trust failure even if stderr mentions certificates",
			res:  &Result{ExitCode: 0, Stderr: []byte("tls: failed to verify certificate")},
			want: false,
		},
		{
			// The exact shape of the 2026-08-17 incident.
			name: "macOS OSStatus trust fault",
			res: &Result{
				ExitCode: 1,
				Stderr: []byte(`Post "https://api.github.com/graphql": tls: ` +
					"failed to verify certificate: x509: OSStatus -26276"),
			},
			want: true,
		},
		{
			name: "bare OSStatus marker",
			res:  &Result{ExitCode: 1, Stderr: []byte("x509: OSStatus -26276")},
			want: true,
		},
		{
			name: "unknown authority (linux / stripped trust store)",
			res:  &Result{ExitCode: 1, Stderr: []byte("x509: certificate signed by unknown authority")},
			want: true,
		},
		{
			// Must NOT reap the daemon: a working credential is being rejected, which
			// is a token problem, not a trust-store problem.
			name: "401 is not a trust failure",
			res:  &Result{ExitCode: 1, Stderr: []byte("HTTP 401: Bad credentials")},
			want: false,
		},
		{
			name: "rate limit is not a trust failure",
			res:  &Result{ExitCode: 1, Stderr: []byte("HTTP 403: API rate limit exceeded")},
			want: false,
		},
		{
			name: "plain network fault is not a trust failure",
			res:  &Result{ExitCode: 1, Stderr: []byte("dial tcp: lookup api.github.com: no such host")},
			want: false,
		},
		{
			name: "ordinary failure with empty stderr",
			res:  &Result{ExitCode: 1},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTrustFailure(tt.res); got != tt.want {
				t.Errorf("IsTrustFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}
