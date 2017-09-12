/*

Package sphyrw implements the Sphyraena Response Writer.

The Sphyraena Response Writer is a superset of the http.ResponseWriter
interface. It wraps around a ResponseWriter and implements the many and
sundry security features that Sphyraena guarantees on responsese, such
as authenticated cookies, the various header-based protections, and
so on.

As I write this, while this conforms to http.ResponseWriter, there's
enough other changes that current Sphyraena is specified in terms of
taking concrete *SphyraenaResponseWriters in the Sphyraena handlers.
Eventually, I expect to break this down a bit, most likely, into
a full-powered interface that does all or nearly all what the
SphyraenaResponseWriter does, one that only offers streaming (useful for
testing code that only needs that functionality), and one that is
simply a SphyraenaResponseWriter cast into an http.ResponseWriter.

*/
package sphyrw

import (
	"net/http"

	"github.com/thejerf/sphyraena/sphyrw/cookie"
)

// In particular, this is necessary to ensure that if Sphyraena wants to
// destroy an invalid, unauthenticated cookie, then user code later wants
// to create the same cookie, the correct single cookie command is sent to
// the browser to avoid confusion, and the intrinsically random way in
// which headers can be output since they fundamentally live in a map.

type SphyraenaResponseWriter struct {
	outCookies       map[string]*cookie.OutCookie
	underlyingWriter http.ResponseWriter
	doneChan         chan struct{}
	responseWritten  bool
	finished         bool
}

// NewSphyraenaResponseWriter creates a new ResponseWriter from the given
// ResponseWriter. This is normally only called by internal code.
//
// The done channel can be passed in to indicate when the response has been
// completed. If nil, it will not be used.
func NewSphyraenaResponseWriter(rw http.ResponseWriter) *SphyraenaResponseWriter {
	return &SphyraenaResponseWriter{
		map[string]*cookie.OutCookie{},
		rw,
		nil,
		false,
		false,
	}
}

func (srw *SphyraenaResponseWriter) Header() http.Header {
	if srw.finished {
		panic("Can't call Header on a Finished SphyraenaResponseWriter")
	}
	return srw.underlyingWriter.Header()
}

func (srw *SphyraenaResponseWriter) Write(b []byte) (int, error) {
	if srw.finished {
		panic("Can't call Write on a Finished SphyraenaResponseWriter")
	}
	if srw.responseWritten {
		return srw.underlyingWriter.Write(b)
	}

	srw.writeResponse()
	return srw.underlyingWriter.Write(b)
}

func (srw *SphyraenaResponseWriter) WriteHeader(code int) {
	if srw.finished {
		panic("Can't call WriteHeader on a Finished SphyraenaResponseWriter")
	}
	srw.writeResponse()
	srw.underlyingWriter.WriteHeader(code)
}

func (srw *SphyraenaResponseWriter) SetCookie(cookie *cookie.OutCookie) {
	if srw.finished {
		panic("Can't call SetCookie on a Finished SphyraenaResponseWriter")
	}
	// deliberately take the latest one, so new user cookies override
	// Sphyraena's cancellation of old cookies
	srw.outCookies[cookie.Name()] = cookie
}

func (srw *SphyraenaResponseWriter) writeResponse() {
	header := srw.underlyingWriter.Header()
	for _, cookie := range srw.outCookies {
		c, err := cookie.Render()
		if err != nil {
			// FIXME: Log this somewhere; couldn't render the cookie
		}
		header.Add("Set-Cookie", c)
	}

	srw.responseWritten = true
}

// SetDoneChan sets the channel that the SphyraenaResponseWriter will use
// to indicate its done-ness to the given channel.
func (srw *SphyraenaResponseWriter) SetDoneChan(done chan struct{}) {
	srw.doneChan = done
}

// Finish completes the request. If in a streaming context, this will
// "release" the current HTTP response while the goroutine continues
// streaming.
//
// Once Finish is called, calling any of the other methods of the
// SphyraenaResponseWriter will result in a panic, as it can only be a
// serious error in logic. It is safe to call Finish multiple times, though
// the latter ones will have no effect.
func (srw *SphyraenaResponseWriter) Finish() {
	if srw.finished {
		return
	}

	if !srw.responseWritten {
		srw.writeResponse()
	}

	if srw.doneChan != nil {
		srw.doneChan <- struct{}{}
	}

	srw.finished = true
}