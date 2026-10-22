package model

import (
	"io"
	"net/http"
	"time"
)

// userAgent is the User-Agent header value sent with every request.
const userAgent = "kbase/0.1"

// responseHeaderTimeout bounds the HANDSHAKE alone: request written →
// response headers received. It lives on the Transport, not on
// http.Client.Timeout, because the latter also covers reading the body and
// so would cap a streamed generation at a wall clock the generation has no
// reason to respect. A server that withholds its headers until the first
// token pays prefill time against this bound, hence the generous value; the
// gap between chunks is bounded separately (streamIdleTimeout).
const responseHeaderTimeout = 2 * time.Minute

// errorBodyLimit caps how much of a non-2xx body is echoed into the
// returned error. A provider that answers with an HTML error page — or a
// runaway one — must not turn a single failed request into a megabyte of
// error string.
const errorBodyLimit = 8 << 10

// newHTTPTransport is the transport both wire clients carry: the process's
// default transport cloned, with a handshake bound.
//
// The shared http.Client carries NO Timeout, deliberately: that field is a
// total round-trip bound including the body read, which would kill exactly
// the long streamed generations this package exists to carry. Bounds are
// placed where they mean something instead — a call-site context deadline
// for a blocking Consult, responseHeaderTimeout for the handshake, and
// streamIdleTimeout for the gap between chunks of a stream.
func newHTTPTransport() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = responseHeaderTimeout
	return &http.Client{Transport: tr}
}

// closeIdleConnections releases h's pooled connections. A verb that opened a
// client calls it before it returns, on every outcome: the pooled connections
// would otherwise outlive the invocation that made them.
func closeIdleConnections(h *http.Client) error {
	if tr, ok := h.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
	return nil
}

// statusError is a non-2xx response as both clients return it: the body read
// (so the connection can be reused) and capped at errorBodyLimit (so a
// provider's HTML error page cannot become a megabyte of error string), with
// any key the server echoed scrubbed out. It takes ownership of resp.Body.
func statusError(resp *http.Response, apiKey string) error {
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
	_ = resp.Body.Close()
	return StatusError{
		Code: resp.StatusCode,
		Body: scrubAuthorization(string(respBody), apiKey),
	}
}
