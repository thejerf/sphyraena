// To be swapped out with whatever is standard with the JS crew.
var strestLogger = console.log;

// url is the URL of the websocket you are trying to connect to for the
// session. In the future this will be something that abstracts away the
// connection bit so it can be polling or whatever is available without
// this session implementation having to worry about it.
//
// sessionID is the session you wish to connect to, if you have an ID in
// hand. If you don't give one, a new one is established. (For now. Pulling
// it from a cookie or something makes sense although our strict cookie
// policies may make that less desirable.)
function stRestSession (url, sessionID) {
    this.url = url;
    this.connecting = false;
    this.connected = false;
    this.buffered = [];
    this.idincr = 1;
    this.requests = {};
    this.resources = {};
    this.sessionID = sessionID;

    this.unsubscribedHandler = function (e) {
        strestLogger("Unhandled event: " + e.data);
    };

    // subscriptionID -> subscription object
    this.subscriptions = {};
}

stRestSession.prototype._connect = function () {
    var self = this;

    this.connecting = true;

    this.ws = new SockJS(this.url + "?stream_id=" + encodeURIComponent(this.sessionID));

    this.ws.onopen = function (event) {
        self.connected = true;
        // need to subscribe back to everything, which means we need the
        // way we requested it in the first place to be hanging around
        // so we can fire it again.
        // no, that creates race conditions for events that happened in the
        // meantime, we need session support on the server.
        self._sendBuffered();
    }

    this.ws.onmessage = function (e) {
        // FIXME: More robust error handling.
        // FIXME: Harmonize .data vs. .message
        var msg = JSON.parse(e.data);
        strestLogger("Incoming: " + e.data);
        if (msg.type == "event") {
            strestLogger("Getting source " + msg.source);
            strestLogger(self.resources);
            var res = self.resources[msg.source];
            // FIXME: handle missing resource
            res.handleEvent(msg.message);
            return;
        }

        if (msg.type == "new_stream_response") {
            strestLogger(self);
            // the ID of the request for the new stream
            var requestID = msg.response_to;
            // the stream ID that we have just created.
            var substream_id = msg.substream_id;
            // FIXME; handle no requestID
            var res = self.requests[requestID];
            strestLogger("Setting console stream " + streamID);
            self.resources[substream_id] = res;
            res.substream_id = substream_id;
            delete self.requests[requestID];

            // FIXME: handle no registered request by that ID
            // FIXME: this could be success or failure, tell difference
            // FIXME: Timeouts.
            strestLogger("Message:");
            strestLogger(msg);
            res.handleSuccess(msg.data);
            return;
        }

        // anything below this point useful?
        var source = msg.source;
        if (source == undefined) {
            self.unsubscribedHandler(e);
            return;
        }
        var subscription = self.subscriptions[source]
        if (subscription) {
            if (msg.response) {
                if (msg.response.streamType == NO_STREAM) {
                    delete self.subscriptions[source];
                }
                return subscription.handleResponse(msg.response);
            } else if (msg.data) {
                return subscription.handleEvent(msg.data);
            } else {
                strestLogger("Subscription " + source + " sent message that is neither response nor event?\n" + e.data)
                return;
            }
        } else {
            strestLogger("Subscription " + source + " has no subscription registered; unsubscribing");
            self.unsubscribe(source);
        }
    };

    this.ws.onclose = function (code, reason, wasclean) {
        self.connected = false;
        self.connecting = false;
        if (Object.keys(self.subscriptions).length != 0) {
            window.setTimeout(function(){self._initiateConnection()}, 5000);
        }
        strestLogger("Web socket closed: code: " + JSON.stringify(code) + ", reason: " + reason  + ", wasclean: " + wasclean)
        // FIXME: Reconnect with exponential backoff
    };
}

stRestSession.prototype._rawSendJSON = function (type, json) {
    if (this.connected) {
        var jsonString = JSON.stringify(json);
        strestLogger("Sending json: " + jsonString);
        this.ws.send(String.fromCharCode(type.length) + type + jsonString);
    } else {
        this.buffered.push([type, json]);
        this._initiateConnection()
    }
}

stRestSession.prototype._sendBuffered = function () {
    for (var idx in this.buffered) {
        var type = this.buffered[idx][0];
        var jsonString = JSON.stringify(this.buffered[idx][1]);
        strestLogger("Sending buffered JSON: " + jsonString);
        this.ws.send(String.fromCharCode(type.length) + type + jsonString);
    }
    this.buffered = [];
}

stRestSession.prototype._initiateConnection = function () {
    if (this.connecting) {
        return;
    }
    this._connect();
}

stRestSession.prototype.substream = function(url, eventHandler, onsuccess, onfail) {
    var resource = new substream(url, this, eventHandler, onsuccess, onfail);
    return resource;
}

// a "substream" represents a particular substream we may be following.
//
// This resource is "dead" in the sense that calling this function does
// nothing; you must "get" or something to it.
//
// eventHandler handles incoming events.
function substream(url, stRestSession, eventHandler, onsuccess, onfail) {
    this.url = url;
    this.stRestSession = stRestSession;
    this.eventHandler = eventHandler;
    this.onsuccess = onsuccess;
    this.onfail = onfail;
    this.substream_id = undefined;
}

substream.prototype.open = function(body, header) {
    console.log("body");
    console.log(body);
    var request_id = this.stRestSession.idincr++;
    // FIXME: Arguably, either POST or a custom verb should be
    // used. Probably the latter, since this never flows through proxies or anything.
    var httpRequest = {method: "POST", url: this.url, header: header, body: JSON.stringify(body),
                       request_id: request_id};
    this.stRestSession.requests[request_id] = this;
    strestLogger("Set requests " + request_id);
    this.stRestSession._rawSendJSON("new_stream", httpRequest);

    // FIXME: timeouts
    strestLogger(this.stRestSession.requests);
}

substream.prototype.send = function(type, msg) {
    this.stRestSession._rawSendJSON("event",
                                    {dest: this.substream_id,
                                     type: type,
                                     message: msg});
}

substream.prototype.handleSuccess = function(response) {
    strestLogger("Got a response, handling...");
    strestLogger(this);
    if (this.onsuccess) {
        strestLogger("Found handler");
        return this.onsuccess(response);
    } else {
        strestLogger(this.url + " got response but no handler provided");
    }
}

substream.prototype.handleFail = function(error) {
    strestLogger("Got an error for the stream, handling...");
    if (this.onfail) {
        strestLogger("Found handler");
        return this,onfail(error);
    } else {
        strestLogger(this.url + " got failure but no handler provided");
    }
}

substream.prototype.handleEvent = function(event) {
    if (this.eventHandler) {
        return this.eventHandler(event);
    } else {
        strestLogger(this.url + " got event but no event handler provided");
    }
}

substream.prototype.unsubscribe = function () {
    this.stRestSession.unsubscribe(this.url)
}

