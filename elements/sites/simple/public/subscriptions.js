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

    this.unsubscribedHandler = function (e) {
        strestLogger("Unhandled event: " + e.data);
    };

    // subscriptionID -> subscription object
    this.subscriptions = {};
}

stRestSession.prototype._connect = function () {
    var self = this;

    this.connecting = true;

    this.ws = new SockJS(this.url);

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
        var msg = JSON.parse(e.data);
        strestLogger("Incoming: " + e.data);
        if (msg.type == "event") {
            strestLogger("Getting stream id " + msg.stream_id);
            var res = self.resources[msg.stream_id];
            // FIXME: handle missing resource
            res.handleEvent(msg.data);
            return;
        }

        if (msg.type == "response") {
            var requestID = msg.request_id;
            // FIXME; handle no requestID
            var res = self.requests[requestID];
            self.resources[msg.stream_id] = res;
            strestLogger("Setting stream id " + msg.stream_id);
            delete self.requests[requestID];

            // FIXME: handle no registered request by that ID
            res.handleResponse(msg.response);
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

stRestSession.prototype._rawSendJSON = function (json) {
    if (this.connected) {
        var jsonString = JSON.stringify(json);
        strestLogger("Sending json: " + jsonString);
        this.ws.send(jsonString);
    } else {
        this.buffered.push(json);
        this._initiateConnection()
    }
}

stRestSession.prototype._sendBuffered = function () {
    for (var idx in this.buffered) {
        var jsonString = JSON.stringify(this.buffered[idx]);
        strestLogger("Sending buffered JSON: " + jsonString);
        this.ws.send(jsonString);
    }
    this.buffered = [];
}

stRestSession.prototype._initiateConnection = function () {
    if (this.connecting) {
        return;
    }
    this._connect();
}

stRestSession.prototype.resource = function(url, responseHandler, eventHandler) {
    return new resource(url, this, responseHandler, eventHandler);
}

// a "resource" represents a resource that we may be streaming.
//
// This resource is "dead" in the sense that calling this function does
// nothing; you must "get" or something to it.
//
// the responseHandler responds to the initial response, which is likely
// enough to be significantly different from an event stream that we need
// to give it its own handler.
//
// eventHandler handles incoming events. If null, the REST request should
// not actually subscribe. Eventually I hope to make the API such that the
// presence or absence of this "event handler" is the only frontend-visible
// distinction between a "streaming" request and a static one.
//
// As for the semantics of a resource: A resource always corresponds to a
// single URL, and that single URL is that resource. Resource authors are
// expected to make this function properly. Resources can rename themselves
// via redirections, including dynamically in the event stream via events
// that the framework itself can process. This URL functions as its ID.
// Due to the fact that URLs can change on a given resource due to
// redirects, we still need an ID internally that we can use to route
// requests to and from this resource stably.
function resource(url, stRestSession, responseHandler, eventHandler) {
    this.url = url;
    this.stRestSession = stRestSession;
    this.responseHandler = responseHandler;
    this.eventHandler = eventHandler;
    this.id = undefined;
}

resource.prototype.get = function(arguments) {
    var header = {};
    var request_id = this.stRestSession.idincr++;
    var httpRequest = {method: "GET", url: this.url, header: header, body: "",
                       request_id: request_id};
    this.stRestSession._rawSendJSON({type: "request", message: httpRequest});

    // FIXME: What to do about outstanding requests?
    this.stRestSession.requests[request_id] = this;
}

resource.prototype.handleResponse = function(response) {
    strestLogger("Got a response, handling...");
    if (this.responseHandler) {
        strestLogger("Found handler");
        return this.responseHandler(response);
    } else {
        strestLogger(this.url + " got response but no handler provided");
    }
}

resource.prototype.handleEvent = function(event) {
    if (this.eventHandler) {
        return this.eventHandler(event);
    } else {
        strestLogger(this.url + " got event but no event handler provided");
    }
}

resource.prototype.unsubscribe = function () {
    this.stRestSession.unsubscribe(this.url)
}

// We also have an open communication channel with the resources we are
// tracking, so we can directly route REST requests to them without the
// need to route.
resource.prototype.postForm = function (params, header) {
    var body = [];

    for (var key in params) {
        body.push(encodeURIComponent(key) + "=" + encodeURIComponent(params[key]))
    }
    var textBody = body.join("&");
    if (header == undefined) {
        header = {};
    }
    header["Content-Type"] = ["application/x-www-form-urlencoded"]

    var httpRequest = {method: "POST", header: header, body: textBody}

    this.stRestSession._rawSendJSON({id: this.id, type: "request", message: httpRequest})
}


