/*

Package cookie represents and renders cookies.

While this is the documentation for the cookie objects, cookies are
generally created through either the session (for authentication) or
the context (for unauthenticated). Since that's probably going to prove
pretty klunky, we'll see how that goes.

The cookie package turns cookies into a default-deny system by making
the most secure cookies the default, and requiring you to selectively back
down protections on target cookies.

By default, a cookie emitted by Sphyraena is:

  * Key/value authenticated to the session that set it, meaning
    neither clients nor MITMs can manipulate the values.
  * HTTPOnly (not visible to JavaScript)
  * Secure (not visible to HTTP)
  * Session-based (destroyed when browser closes)
  * Strictly standards-compliant (checked for conformance to the
    strictest reading of RFC 6265).
  * Path set to /.
  * The SameSite flag will be set to Strict.

As other security features are added to cookies, they will be added in by
default here.

In addition, this takes over cookie rendering and parsing duties from
net/http. Rendering is taken over because net/http attempts to "fix up"
outgoing cookies. For instance, at least as of this writing, if
net/http decides it doesn't like the "domain" value of the cookie,
it simply removes it, which can have serious security implications.
This is too dangerous; illegal cookies should be rejected, not given
unexpectedly-larger scope. Parsing is taken over so Sphyraena can
implement authentication of cookies, verifying that cookies were issued
by a given session.

Finally, incoming and outgoing cookies are given separate types, to
avoid the issues with trying to straddle all the use cases with one type
when types are cheap
(https://github.com/golang/go/issues/7243#issuecomment-66090884).

The OutCookie is configured via the "functional options" pattern:
http://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
The functions that either are func(*OutCookie) or return one are
functional options.

Cookie encryption is deliberately NOT supported, and will not be
supported. It is too frequently misused and generally provides only the
illusion of security. If you do not want a client to see a value, do not
send it to them; stick it in the session.

The best current description of cookies is available from RFC 6265.
http://tools.ietf.org/html/rfc6265 But most browsers accept spaces and
commas in the cookie value, which the NonstandardCookie reflects.

*/
package cookie

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/thejerf/abtime"
	"github.com/thejerf/sphyraena/secret"
)

const (
	// the length of the hmac name signature in base64
	hmaclength = 43
	// the length of the entire postfix if authed, which is hmaclength +
	// len(signSuffix)
	authedLength = 55

	signSuffix = "__!sauthed!_"

	authenticated   = true
	unauthenticated = false
)

// Strict can be passed to the SameSite option to set the SameSite cookie
// flag to Strict, which is the default.
var Strict = CookieStrictness{0}

// Lax can be passed to the SameSite option to set the SameSite cookie flag
// to Lax.
var Lax = CookieStrictness{1}

// NoSameSiteSetting can be passed to the SameSite option to entirely
// remove the SameSite setting from the cookie.
var NoSameSiteSetting = CookieStrictness{2}

type CookieStrictness struct {
	strictness byte
}

func (cs CookieStrictness) render() string {
	switch cs.strictness {
	case 0:
		return "SameSite=Strict"
	case 1:
		return "SameSite=Lax"
	default:
		return ""
	}
}

// A Option modifies an OutCookie in the given manner.
type Option func(*OutCookie) error

var t abtime.AbstractTime = abtime.NewRealTime()

type errCookieInvalid struct {
	name   string
	reason string
}

func (eci *errCookieInvalid) Error() string {
	return fmt.Sprintf("Can't create Cookie '%s' because %s", eci.name, eci.reason)
}

// A OutCookie represents an HTML cookie which will be sent out via a header.
// Note this can only be sent via a true HTTP request, not via a stream,
// though I expect to fix this eventually.
type OutCookie struct {
	name          string
	value         string
	authenticator secret.Authenticator

	hasExpires bool
	maxAge     time.Duration
	expires    time.Time

	path               string
	domain             string
	clientAccess       bool
	insecure           bool
	sameSiteStrictness CookieStrictness
}

// Name returns the name of the outcookie.
func (oc *OutCookie) Name() string {
	// We don't really want to expose anything else, particulary we don't
	// want to expose the value so as to tempt people into writing their
	// own Render functions, but this is needed by the SphyraenaResponseWriter.
	return oc.name
}

// An InCookie represents an incoming cookie. It consists of its Name, its
// Value, and a boolean indicating whether this cookie has been origin
// authenticated.
type InCookie struct {
	// all privatized so that external users can't mess with this; in
	// particular, hacking on a name or value invalidates the authentication.
	name          string
	value         string
	authenticated bool
}

// GoString implements the fmt.GoStringer interface.
func (ic *InCookie) GoString() string {
	return fmt.Sprintf("%s=%s/authenticated=%v", ic.name, ic.value, ic.authenticated)
}

// Name returns the name of the incoming cookie.
func (ic *InCookie) Name() string {
	return ic.name
}

// Value returns the value of the incoming cookie.
func (ic *InCookie) Value() string {
	return ic.value
}

// Authenticated returns whether this cookie was securely authenticated as
// sourcing from the Session.
func (ic *InCookie) Authenticated() bool {
	return ic.authenticated
}

// Note that InCookies are read only.

// InCookies wraps incoming cookies and provides mediated, default-safe
// access to the cookies.
type InCookies struct {
	// due to the restrictions on string names, unicode normalization is
	// irrelevant; these must already be ASCII-only.
	cookies map[string]*InCookie
}

// Count returns the number of cookies in this InCookies set.
func (ic *InCookies) Count() int {
	if ic == nil {
		return 0
	}
	return len(ic.cookies)
}

func (ic *InCookies) addCookie(name, value string, validated bool) {
	ic.addInCookie(&InCookie{name, value, validated})
}

func (ic *InCookies) addInCookie(in *InCookie) {
	if ic.cookies == nil {
		ic.cookies = map[string]*InCookie{}
	}

	oldCookie := ic.cookies[in.name]
	if oldCookie == nil {
		ic.cookies[in.name] = in
		return
	}

	// If this is a validated cookie, and the old one was not, take the
	// new one.
	if !oldCookie.authenticated && in.authenticated {
		ic.cookies[in.name] = in
		return
	}
	// otherwise, the status matches, in which case, keep the old one
	return
}

// Get will retrieve an authenticated cookie by name. Non-authenticated
// cookies will not be returned, resulting in nil. To retrieve
// non-authenticated cookies, use GetPossiblyUnauthenticated.
func (ic *InCookies) Get(name string) *InCookie {
	if ic == nil {
		return nil
	}
	cookie := ic.cookies[name]
	if cookie == nil {
		return nil
	}
	if !cookie.authenticated {
		return nil
	}
	return cookie
}

// GetPossiblyUnauthenticated will retrieve the given InCookie by name
// regardless of its validation status.
func (ic *InCookies) GetPossiblyUnauthenticated(name string) *InCookie {
	if ic == nil {
		return nil
	}
	return ic.cookies[name]
}

// GoString gives the InCookies a nice #%v representation.
func (ic InCookies) GoString() string {
	var buf bytes.Buffer

	buf.Write([]byte("[InCookies: {"))
	written := false
	names := []string{}
	for name := range ic.cookies {
		names = append(names, name)
	}
	// stability for the unit tests, if nothing else
	sort.Sort(sort.StringSlice(names))
	for _, name := range names {
		if written {
			buf.Write([]byte(", "))
		}
		written = true
		fmt.Fprintf(&buf, "%#v", ic.cookies[name])
	}
	buf.Write([]byte("}]"))

	return buf.String()
}

// ParseCookies parses the incoming cookies.
//
// This is necessary because only Sphyraena can correctly unwrap
// the authenticated names. This will never return nil, which can be used
// to distinguish between parsed-but-empty cookies and unparsed cookies.
//
// The second return value is the set of cookies that failed
// authentication. Sphyraena will automatically serve Set-Cookie headers to
// attempt to expire these cookies.
//
// The last value is nil if there was no valid session ID, or a pointer to
// a string containing the session ID if it is valid.
//
// FIXME: This sort of overpriviliges cookie-based authentication. The act
// of authenticating cookies probably needs to be further unwrapped from
// this somehow, perhaps handing off all the apparently-authenticated
// cookies to something?
func ParseCookies(cookies []string, authUnwrappers secret.AuthenticationUnwrappers) (*InCookies, []string) {
	// copied from net/http/cookie.go, modified into near-unrecognizability
	result := &InCookies{}
	failedCookies := []string{}

	// here's the plan for this function:
	// * Parse each incoming cookie.
	//   * if it could be a session cookie, put it to the side.
	//   * if it could be an authenticated cookie, put it to the side.
	// * Determine which session, if any, is valid.
	// * Once a session is selected, figure out which authenticated cookies
	//   are valid.
	// * Reject all the others, and return them as things that need to be
	//   erased.
	//
	// Erasing the invalid cookies isn't so much about security, since if
	// they never make it past this code there isn't much they can do; it's
	// just about cleanliness. The downside of authenticating

	if len(cookies) == 0 {
		return result, failedCookies
	}

	hmaced := []*InCookie{}
	var possiblySession *InCookie

	for _, line := range cookies {
		parts := strings.Split(strings.TrimSpace(line), ";")
		for i := 0; i < len(parts); i++ {
			parts[i] = strings.TrimSpace(parts[i])
			if len(parts[i]) == 0 {
				continue
			}
			name, val := parts[i], ""
			if j := strings.Index(name, "="); j >= 0 {
				name, val = name[:j], name[j+1:]
			}

			if !isCookieNameLoose([]byte(name)) {
				continue
			}

			if len(val) > 1 && val[0] == '"' && val[len(val)-1] == '"' {
				val = val[1 : len(val)-1]
			}
			if !isCookieValueLoose([]byte(val)) {
				continue
			}

			if isAuthed(val) {
				// FIXME: ought to either use something else or make this
				// configurable at the Sphyraena level so it can get along
				// with other things that may insist on this.
				if name == "session" {
					// should the browser send more than one, we will take
					// the first. Since this is authenticated this shouldn't
					// do anything weird; worst case should be the session
					// value is invalid and everything gets cleared.
					if possiblySession == nil {
						possiblySession = &InCookie{name: name, value: val}
					}
				} else {
					hmaced = append(hmaced, &InCookie{name: name, value: val})
				}
			} else {
				result.addCookie(name, val, unauthenticated)
			}
		}
	}

	nukeAllAuthedCookies := func() {
		for _, cookie := range hmaced {
			failedCookies = append(failedCookies, cookie.name)
		}
		failedCookies = append(failedCookies, "session")
	}

	// now, having dealt with parsing the cookies, we need to see if one of
	// these is a session cookie. We have a bit of a chicken&egg problem
	// because the session's name is authenticated by the secret ID which
	// we get from the session ID.
	if possiblySession != nil {
		// penetrate the signing abstraction
		possibleSessionID := possiblySession.value[:len(possiblySession.value)-authedLength]
		authUnwrapper, err := authUnwrappers.GetAuthenticationUnwrapper(possibleSessionID)

		if err != nil {
			// With no session to authenticate these values, they're all
			// trash.
			nukeAllAuthedCookies()
		} else {
			// only thing the _ could be is the session ID we already extracted.
			_, err := authUnwrapper.UnwrapAuthentication(
				[]byte("session"),
				[]byte(possiblySession.value),
			)
			if err != nil {
				nukeAllAuthedCookies()
			} else {
				result.addCookie("session", string(possibleSessionID), true)
				for _, cookie := range hmaced {
					val, err := authUnwrapper.UnwrapAuthentication(
						[]byte(cookie.name),
						[]byte(cookie.value),
					)
					if err != nil {
						failedCookies = append(failedCookies, cookie.name)
					} else {
						cookie.name = string(val)
						result.addInCookie(cookie)
					}
				}
			}
		}
	}

	return result, failedCookies
}

// isAuthed retursn if the given value as a string may be an authenticated
// cookie value.
//
// Should this falsely fire due to a remote attacker doing something mean,
// all that should happen is that the cookie subsequently fails the check
// and is therefore removed from the cookie pile. This should be
// indistinguishable from a client never sending it, which is a thing we
// can't prevent anyhow, so any resulting insecurity would be "real" anyhow
// and not caused by Sphyraena.
func isAuthed(s string) bool {
	if len(s) < authedLength {
		return false
	}

	sig := s[len(s)-authedLength:]
	if sig[:len(signSuffix)] != signSuffix {
		return false
	}

	if !maybeSignature(sig[len(signSuffix):]) {
		return false
	}

	return true
}

// NewOut creates a cookie.
//
// The RFC6265 specification for cookie name is enforced, which requires
// that the name of the cookie consist of US ASCII characters that are
// not control characters or separators, as defined by the "token"
// construction in RFC2616: http://tools.ietf.org/html/rfc2616#section-2.2
// (text search for "token" after that). An error will be returned if the
// name is invalid.
//
// The RFC6265 specification for cookie values is enforced, which requires
// that the cookie values consist only of "cookie octets", which are US
// ASCII characters excluding control characters, whitespace, double
// quotes, comma, semicolon, and backslash. An error will be returned if
// the name is invalid.
//
// This is a "pure function", so if you pass in only constant values, you
// can be assured no error will come out.
func NewOut(
	name string,
	value string,
	authenticator secret.Authenticator,
	options ...Option,
) (*OutCookie, error) {
	return newcookie(true, name, value, authenticator, options...)
}

// NewNonstandardOut creates a non-standard cookie out.
//
// The RFC6265 specification for cookie name remains enforced.
//
// The RFC6265 specification for values is extended to include comma and space.
//
// This function may get looser in the future if it proves necessary. The
// idea is that if you must do something insecure, this function will be
// the one that takes the security hit, leaving "NewOut" a secure function.
//
// See https://hackerone.com/reports/14883 for an exciting unexpected
// consequence of allowing commas in the value (when combined with other
// things).
func NewNonstandardOut(
	name string,
	value string,
	authenticator secret.Authenticator,
	options ...Option,
) (*OutCookie, error) {
	return newcookie(false, name, value, authenticator, options...)
}

func newcookie(
	strict bool,
	name string,
	value string,
	authenticator secret.Authenticator,
	options ...Option,
) (*OutCookie, error) {
	if name == "" {
		return nil, &errCookieInvalid{name, "no name given"}
	}
	for _, c := range []byte(name) {
		if c < 32 || c >= 128 {
			return nil, &errCookieInvalid{name, "the name contains invalid characters"}
		}
		switch c {
		// set of chars extracted from RFC2616
		case ' ', '(', ')', '<', '>', '@', ',', ';', ':', '\\', '"', '/', '[', ']', '?', '=', '{', '}':
			return nil, &errCookieInvalid{name, "the name contains invalid characters"}
		}
	}

	if strict {
		if !isCookieValueStrict([]byte(value)) {
			return nil, &errCookieInvalid{name, "the value contains invalid characters"}
		}
	} else {
		if !isCookieValueLoose([]byte(value)) {
			return nil, &errCookieInvalid{name, "the value contains invalid characters"}
		}
	}

	cookie := &OutCookie{name: name, value: value, authenticator: authenticator}

	for _, option := range options {
		err := option(cookie)
		if err != nil {
			return nil, err
		}
	}

	return cookie, nil
}

func isCookieNameLoose(value []byte) bool {
	for _, c := range value {
		if !cookieNameChars[c] {
			return false
		}
	}
	return true
}

func isCookieValueStrict(value []byte) bool {
	for _, c := range value {
		if c < 32 || c >= 128 {
			return false
		}
		switch c {
		case ' ', '"', ',', ';', '\\':
			return false
		}
	}
	return true
}

func isCookieValueLoose(value []byte) bool {
	for _, c := range value {
		if c < 32 || c >= 128 {
			return false
		}
		switch c {
		case '"', ';', '\\':
			return false
		}
	}
	return true
}

// Render will render the cookie into the string form suitable for use in
// HTTP. This is probably only used by Sphyraena itself.
//
// Errors can only occur when the authenticator returns an error. If this
// is an unauthenticated cookie, no errors can occur.
func (c *OutCookie) Render() (string, error) {
	// this is safe because the name and the value can only be set via
	// mechanisms that will validate them.
	var v = c.value
	if len(v) > 0 {
		if v[0] == ' ' || v[0] == ',' || v[len(v)-1] == ' ' || v[len(v)-1] == ',' {
			v = `"` + v + `"`
		}
	}

	name := []byte(c.name)
	if c.authenticator != nil {
		signed, err := c.authenticator.Authenticate(name, []byte(c.value))
		if err != nil {
			return "", err
		}
		v = string(signed)
	}

	chunks := []string{fmt.Sprintf("%s=%s", string(name), v)}

	if c.maxAge != time.Duration(0) {
		seconds := c.maxAge / time.Second
		chunks = append(chunks, fmt.Sprintf("Max-Age=%d", seconds))
	}
	if c.hasExpires {
		if !c.expires.IsZero() {
			chunks = append(chunks, fmt.Sprintf("Expires=%s", c.expires.UTC().Format(http.TimeFormat)))
		} else {
			chunks = append(chunks, "Expires=Fri, 02-Jan-1970 00:00:01 GMT")
		}
	}
	if c.path != "" {
		chunks = append(chunks, "Path="+c.path)
	} else {
		chunks = append(chunks, "Path=/")
	}
	if c.domain != "" {
		chunks = append(chunks, "Domain="+c.domain)
	}
	if !c.clientAccess {
		chunks = append(chunks, "HttpOnly")
	}
	if !c.insecure {
		chunks = append(chunks, "Secure")
	}
	sameSite := c.sameSiteStrictness.render()
	if sameSite != "" {
		chunks = append(chunks, sameSite)
	}

	return strings.Join(chunks, ";"), nil
}

// Delete instructs the user to delete the cookie by setting the cookie's
// expire time deep in the past. This will also set the value to the empty string.
func Delete(c *OutCookie) error {
	c.maxAge = time.Duration(0)
	c.expires = time.Time{}
	c.hasExpires = true
	c.authenticator = nil
	c.value = ""
	return nil
}

// Duration is the time for the cookie to be set.
//
// This results in both Max-Age and Expires being sent. Browsers that use
// only Expires are subject to the whimsy of the user's clock. Be wary of
// trying to set these too short. The resulting Expires will be based on
// the server's clock, of course.
//
// An nil *OutCookie and an error will result if the the duration is less than
// one second (which will get rounded to zero, which is presumed not to be
// the intent), or if the resulting Expires calculation's year exceeds
// 2038. This will presumably at some point be lifted, but it's still a bit
// of a bad idea to send out cookies beyond that.
func Duration(d time.Duration) Option {
	return func(c *OutCookie) error {
		var reasonInvalid error
		if d < 0 {
			reasonInvalid = &errCookieInvalid{c.name, "duration was negative (use .Delete to delete deliberaterly)"}
		}
		if d < time.Second {
			reasonInvalid = &errCookieInvalid{c.name, "duration was set to less than one second"}
		}
		if t.Now().Add(d).Year() >= 2038 {
			reasonInvalid = &errCookieInvalid{c.name, "cookie's duration is too long (use .Forever() to set deliberate long-lived cookie)"}
		}

		if reasonInvalid != nil {
			return reasonInvalid
		}

		c.hasExpires = true
		c.expires = t.Now().Add(d)
		c.maxAge = d

		return nil
	}
}

// Session turns this into a session cookie, which is accomplished by not
// sending any expires time.
func Session(c *OutCookie) error {
	c.hasExpires = false
	c.maxAge = time.Duration(0)
	return nil
}

// SameSite will configure the SameSite value on the cookie. As I write
// this this is only in Chrome, but I expect it is very likely that it will
// go out to other browsers.
//
// See: https://tools.ietf.org/html/draft-west-first-party-cookies-07
func SameSite(cs CookieStrictness) Option {
	return func(c *OutCookie) error {
		c.sameSiteStrictness = cs
		return nil
	}
}

// Forever labels the cookie as the closest to "forever" you can get.
func Forever(c *OutCookie) error {
	c.hasExpires = true
	c.expires = time.Unix(2147483647, 0)
	c.maxAge = time.Duration(0)
	return nil
}

// Path sets the path of the cookie.
//
// This is primarily included because it is something cookies can do, and
// it would not be correct to claim "cookie support" if this were not
// present. However, it is at least a security code smell to use this, as
// paths in cookies are much more likely to cause confusion than to enhance
// security.
//
// http://lcamtuf.blogspot.com/2010/10/http-cookies-or-how-not-to-design.html
//
// An empty string passed to this function, or not calling it, will
// result in a path of "/". This should basically result in cookies
// following the same-origin policy.
//
// An error will be returned if the path does not conform to RFC6265's
// specification for what a path can be.
func Path(path string) Option {
	return func(c *OutCookie) error {
		for _, b := range []byte(path) {
			if b < 32 || b >= 128 || b == '"' || b == ';' || b == '\\' {
				return &errCookieInvalid{c.name, "path is illegal"}
			}
		}
		c.path = path
		return nil
	}
}

// Domain sets the domain of the cookie.
//
// This is actually quite tricky to get right. There is no safe way to set
// a cookie for just the current domain, since still-in-use IE versions
// do not have a way to do that at all, so you must always plan for the
// possibility that the cookies will be sent to subdomains.
//
// This suggests this is a reason to consider keeping the
// "www." prefix in your web site, so that if you do ever host untrusted
// content you can still put it on another domain:
// http://erik.io/blog/2014/03/04/definitive-guide-to-cookie-domains/
//
// Thus, in general, the best way to invoke this function is not
// to. If your app uses this, I strongly recommend careful testing of
// both where the cookie goes and where it doesn't go, to ensure it is
// working as designed.
//
// Setting the empty string will result in no domain value being sent out.
func Domain(domain string) func(*OutCookie) error {
	return func(c *OutCookie) error {
		if !isCookieDomainName(domain) {
			return &errCookieInvalid{c.name, "domain is illegal"}
		}
		c.domain = domain
		return nil
	}
}

// copied from net/http/cookie.go
func isCookieDomainName(s string) bool {
	if len(s) == 0 {
		return false
	}
	if len(s) > 255 {
		return false
	}

	if s[0] == '.' {
		// A cookie a domain attribute may start with a leading dot.
		s = s[1:]
	}
	last := byte('.')
	ok := false // Ok once we've seen a letter.
	partlen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		default:
			return false
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			// No '_' allowed here (in contrast to package net).
			ok = true
			partlen++
		case '0' <= c && c <= '9':
			// fine
			partlen++
		case c == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partlen++
		case c == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partlen > 63 || partlen == 0 {
				return false
			}
			partlen = 0
		}
		last = c
	}
	if last == '-' || partlen > 63 {
		return false
	}

	return ok
}

// ClientCanRead allows the client's Javascript to see the cookie.
//
// This sends the cookie without the HTTPOnly flag, renamed to make it more
// clear what that means. This also turns off authentication, because the
// client can't authenticate the cookie.
func ClientCanRead(c *OutCookie) error {
	c.clientAccess = true
	c.authenticator = nil
	return nil
}

// Insecure allows the cookie to be sent over HTTP, in addition to HTTPS.
//
// This is the "secure" flag on a cookie, with a method name designed to
// make security review easier.
func Insecure(c *OutCookie) error {
	c.insecure = true
	return nil
}

// this creates slices that can be used to lookup legal characters
func legalslice(s string) (slice [256]bool) {
	for _, v := range []byte(s) {
		slice[v] = true
	}
	return
}

var cookieNameChars = legalslice("!#$%&'*+-.0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ^_`abcdefghijklmnopqrstuvwxyz|~")
var base64URI = legalslice("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ$%")

func maybeSignature(s string) bool {
	if len(s) != hmaclength {
		return false
	}
	for _, c := range []byte(s) {
		if !base64URI[c] {
			return false
		}
	}
	return true
}
