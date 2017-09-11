/*

Package sockjs provides an implementation of Streaming REST streams based
on SockJS.

SockJS is a javascript library that attempts to provide a websocket-like
API for a wide array of browsers and connection
methods. https://github.com/igm/sockjs-go/ implements a Go SockJS server.

This provides a streaming REST handler that implements an endpoint that
can be configured via the router to provide the concrete streaming for
a stream.

FIXME: Vendor the SockJS Go server?

*/
package sockjs
