package sample

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/sphyrw"
)

// JSONForwarder defines a protocol for forwarding web requests to external
// handlers in the form of a JSON request (see the WrappedRequest below)
// and JSON response over a socket. JSONForwarders receive the request
// coming in, fully extract out what it can, and then open a socket to the
// target net/host via net.Dial and pass along the JSON request. It waits
// for a JSON answer (as seen in the JSONResponse object), and uses that to
// provide the response.
//
// This is done via a simple protocol; four bytes in BigEndian order that
// describe how much JSON is coming, then that much JSON. The remote end is
// expected to reply in kind.
//
// The advantage of this is that it has a very good bang/buck ratio, and
// decent performance if the remote service is not too far away. The
// disadvantages are: No streaming or reading support (this fully reads
// bodies, it can't pass a stream along), and it can't carry any response
// that can't be fit into JSON. So, for instance, if the remote site is
// going to serve up an image, it has to Base64 it, where the Go
// encoding/json will turn it back into a []byte, but there's no getting
// around this step. This isn't necessarily industrial-strength robust, but
// it can be a great prototype tool, and if the previous disadvantages
// never come up, nothing stops you from shipping it.
type JSONForwarder struct {
	// The port to speak to
	Net  string
	Host string
}

// Now that in Go 1.5 *http.Request contains a channel, we can not ship it
// to clients directly.

// WrappedRequest is a request based on the incoming Request that is safe
// to send via JSON. Some of the elements are copied by reference from the
// request, so bear in mind manipulating the headers of WrappedRequest will
// still manipulate the original *http.Request. (Since Requests are
// generally consumed, this seems unlikely to be a problem.)
//
// Values that have already been consumed just to generate this value are
// not passed along; for instance, TransferEncoding is not relevant here.
type WrappedRequest struct {
	Method        string      `json:"method"`
	URL           *url.URL    `json:"url"`
	Proto         string      `json:"proto"`
	ProtoMajor    int         `json:"proto_major"`
	ProtoMinor    int         `json:"proto_minor"`
	Header        http.Header `json:"headers"`
	Body          string      `json:"body"`
	ContentLength int64       `json:"content_length"`
	Host          string      `json:"host"`
	Form          url.Values  `json:"form"`
	PostForm      url.Values  `json:"post_form"`
	RemoteAddr    string      `json:"remote_addr"`
}

func WrapRequest(req *http.Request) *WrappedRequest {
	// We do not want the handling for PUT or POST, because we're
	// submitting JSON regardless.
	if req.Method == "GET" {
		req.ParseForm()
	}

	var body string
	if req.Body != nil {
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}
		body = string(b)
	}

	return &WrappedRequest{
		Method:        req.Method,
		URL:           req.URL,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Header:        req.Header,
		Body:          body,
		ContentLength: req.ContentLength,
		Host:          req.Host,
		Form:          req.Form,
		PostForm:      req.PostForm,
		RemoteAddr:    req.RemoteAddr,
	}
}

// JSONResponse is what encode/json will be used to decode the response
// into. It is then fed to the Response in the obvious manner.
type JSONResponse struct {
	Headers  map[string][]string `json:"headers"`
	Body     string              `json:"body"`
	Response int                 `json:"response"`
}

// This implements the ServeStreaming method of the StreamingREST interface.
//
// This does Sphyraena-specific functionality, like determining the
// logged-in user.
func (jf *JSONForwarder) ServeStreaming(rw *sphyrw.SphyraenaResponseWriter, context *context.Context) {
	fmt.Println("Hello?")

	// if this comes back blank, it will not be passed in
	userID, _ := context.Session().Identity().UniqueID()

	response := jf.HandleReq(context.Request, context.PrecedingPath, userID)

	headers := rw.Header()
	for key, value := range response.Headers {
		headers[key] = value
	}
	if response.Response != 0 {
		rw.WriteHeader(response.Response)
	}
	_, err := rw.Write([]byte(response.Body))
	if err != nil {
		panic(err)
	}
}

func (jf *JSONForwarder) MayStream() bool {
	return false
}

// HandleReq forwards the request to the JSONForwarder.
//
// It is perfectly legal to use this with internal processes that can
// construct a legal *http.Request.
func (jf *JSONForwarder) HandleReq(req *http.Request, locforward string, userID string) JSONResponse {
	wreq := WrapRequest(req)

	// Purge any incoming X-Sphyraena-* headers that may have been
	// incoming, so the JSON consumer has assurance this is from the server.
	for header := range req.Header {
		if len(header) > 12 && strings.ToLower(header[:12]) == "x-sphyraena-" {
			delete(req.Header, header)
		}
	}

	wreq.Header["X-Sphyraena-Location-Forward"] = []string{locforward}
	if userID != "" {
		wreq.Header["X-Sphyraena-Authenticated-User"] = []string{userID}
	}

	jsonReq, err := json.Marshal(wreq)
	if err != nil {
		panic(err)
	}

	conn, err := net.Dial(jf.Net, jf.Host)
	if err != nil {
		panic(err)
	}

	err = binary.Write(conn, binary.BigEndian, uint32(len(jsonReq)))
	if err != nil {
		panic(err)
	}
	_, err = conn.Write(jsonReq)
	if err != nil {
		panic(err)
	}

	var size uint32
	err = binary.Read(conn, binary.BigEndian, &size)
	if err != nil {
		panic(err)
	}

	buf := make([]byte, size)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		panic(err)
	}

	response := JSONResponse{}
	err = json.Unmarshal(buf, &response)
	if err != nil {
		fmt.Println("I couldn't handle:", string(buf), "|")
		panic(err)
	}
	return response
}
