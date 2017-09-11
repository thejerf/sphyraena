/*

Package utf8stream provides a generic wrapper around any underlying
transport that is capable of sending and receiving discrete blocks
of UTF8 data and turns it into Streaming REST transport.

While this is particularly intended for very generic websocket/web polling
support as provided by sockjs, it turns out most of that support is very
generic. It's also very easy to implement this protocol as a client, too,
by simply sending Length/Value strings, which is also provided as a
simple-but-usable implementation of streaming support that can be
surprisingly easy to implement in generic clients.

This defines a stream that permits simple HTTP requests to be made
inline, responses to be matched up with their requests, and streaming
events to be received.

*/
package utf8stream
