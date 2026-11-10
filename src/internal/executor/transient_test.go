package executor

import "testing"

func TestIsTransientNetworkMatchesWatchTimeout(t *testing.T) {
	msg := []byte(`Post "https://api.github.com/graphql": read tcp 192.168.3.52:58972->20.87.245.6:443: read: operation timed out`)
	if !IsTransientNetwork(msg) {
		t.Fatal("expected the captured watch timeout to be transient")
	}
}

func TestIsTransientNetworkIgnoresSemanticCheckFailure(t *testing.T) {
	if IsTransientNetwork(nil) {
		t.Fatal("empty output is not a transport fault")
	}
	if IsTransientNetwork([]byte("gate\tfail\n")) {
		t.Fatal("a failed check listing is not a transport fault")
	}
	if IsTransientNetwork([]byte("HTTP 401: Bad credentials")) {
		t.Fatal("auth failure must not be retried as a transient network error")
	}
}

func TestIsUncacheableFailureIncludesAuthAndTransport(t *testing.T) {
	if !IsUncacheableFailure([]byte("HTTP 401: Bad credentials")) {
		t.Fatal("expected auth failure to be uncacheable")
	}
	if !IsUncacheableFailure([]byte("read: operation timed out")) {
		t.Fatal("expected timeout to be uncacheable")
	}
	if IsUncacheableFailure([]byte("gate\tpending\n")) {
		t.Fatal("a pending check listing is a completed read")
	}
}
