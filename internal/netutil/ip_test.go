package netutil_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jsirianni/cloudflare-go/internal/netutil"
	"github.com/stretchr/testify/require"
)

type stubTransport struct {
	status  int
	body    string
	err     error
	delay   time.Duration
	bodyErr error
}

func (s stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	var rc io.ReadCloser
	if s.bodyErr != nil {
		rc = io.NopCloser(errReader{s.bodyErr})
	} else {
		rc = io.NopCloser(strings.NewReader(s.body))
	}
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return &http.Response{
		StatusCode: s.status,
		Status:     fmt.Sprintf("%d %s", s.status, http.StatusText(s.status)),
		Body:       rc,
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestDiscoverIPv4ViaIpify_TableDriven(t *testing.T) {
	type testCase struct {
		name      string
		transport stubTransport
		ctx       func() (context.Context, context.CancelFunc)
		wantIP    string
		wantErr   string
	}

	ctxOK := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 2*time.Second)
	}
	ctxCanceled := func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, func() {}
	}
	ctxDeadline := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 10*time.Millisecond)
	}

	cases := []testCase{
		{
			name:      "success IPv4",
			transport: stubTransport{status: http.StatusOK, body: "203.0.113.42"},
			ctx:       ctxOK,
			wantIP:    "203.0.113.42",
		},
		{
			name:      "non-2xx status",
			transport: stubTransport{status: http.StatusInternalServerError, body: "error"},
			ctx:       ctxOK,
			wantErr:   "500",
		},
		{
			name:      "invalid ip string",
			transport: stubTransport{status: http.StatusOK, body: "hello"},
			ctx:       ctxOK,
			wantErr:   "invalid IPv4 response",
		},
		{
			name:      "ipv6 not accepted",
			transport: stubTransport{status: http.StatusOK, body: "2001:db8::1"},
			ctx:       ctxOK,
			wantErr:   "invalid IPv4 response",
		},
		{
			name:      "empty body",
			transport: stubTransport{status: http.StatusOK, body: ""},
			ctx:       ctxOK,
			wantErr:   "invalid IPv4 response",
		},
		{
			name:      "body read error",
			transport: stubTransport{status: http.StatusOK, bodyErr: errors.New("boom")},
			ctx:       ctxOK,
			wantErr:   "boom",
		},
		{
			name:      "context canceled before request",
			transport: stubTransport{status: http.StatusOK, body: "203.0.113.42"},
			ctx:       ctxCanceled,
			wantErr:   context.Canceled.Error(),
		},
		{
			name:      "context deadline exceeded during roundtrip",
			transport: stubTransport{status: http.StatusOK, body: "203.0.113.42", delay: 50 * time.Millisecond},
			ctx:       ctxDeadline,
			wantErr:   context.DeadlineExceeded.Error(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.ctx()
			defer cancel()
			client := &http.Client{Transport: tc.transport}
			ip, err := netutil.DiscoverIPv4ViaIpify(ctx, client)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantIP, ip)
		})
	}
}
