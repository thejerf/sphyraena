/*

Package hole provides support for opening security holes in HTTP responses.

Sphyraena makes the web default-deny. In order to do things, you must open
"holes" in that default deny policy. Perhaps the term "hole" is a bit...
aggressive... but I tried out several other nouns and none of them really
seemed to fit.

Besides, I think it sets the tone correctly. You may need to open "holes"
in your security to operate, but they should be viewed as holes, not merely
"turning on features". Better a bit too paranoid than a bit too little.

*/
package hole

import "net/http"

// security tracks the security requests for this connection. It defaults
// to total security, and monoidally backs down the security as requests
// come in.
type security struct {
	allowBrowserTypeGuessing bool
}

func (s *security) applyHoles(holes []SecurityHole) {
	for _, hole := range holes {
		hole.applySecurityHole(s)
	}
}

// ApplySecurityHeaders takes the given SecurityHoles and applies the
// correct HTML headers to implement the given policy.
func ApplySecurityHeaders(headers http.Header, holes SecurityHoles) {
	sec := security{}
	sec.applyHoles(holes)

	if !sec.allowBrowserTypeGuessing {
		headers.Set("X-Content-Type-Options", "nosniff")
	}
}

// A SecurityHole is a request to lower the security on a given
// response. Applying security policy is done by starting with the base
// "default deny" policy and applying all the relevant holes.
type SecurityHole interface {
	applySecurityHole(*security)
}

type allowBrowserTypeGuessing struct{}

func (acs allowBrowserTypeGuessing) applySecurityHole(s *security) {
	s.allowBrowserTypeGuessing = true
}

// AllowBrowserTypeGuessing returns a SecurityLoosening that allows browers
// to guess the type of the content coming in.
//
// In HTTP terms, this prevents Sphyraena from emitting
// X-Content-Type-Options: nosniff.
//
// In security terms, this is dangerous because browsers can be convinced
// to incorrectly sniff types, and may unexpectedly decide a page is HTML
// and allow script execution.
func AllowBrowserTypeGuessing() SecurityHole {
	return allowBrowserTypeGuessing{}
}

// The NoHole is something that conforms to the SecurityHole
// interface, but does not result in any opening of security when
// applied.
//
// This is useful to create functions that unconditionally return
// something of the type SecurityHole for simplicity, but may sometimes
// choose to return "nothing".
func NoHole() SecurityHole {
	return noHole{}
}

type noHole struct{}

func (nh noHole) applySecurityHole(s *security) {
	// deliberately do nothing
	return
}

// SecurityHoles is simply a slice type of SecurityHole that is augmented
// with the method to turn it into a SecurityHole itself.
//
// If you work at it, you can use this to create cycles, e.g.
//
//    holes := SecurityHoles{}
//    holes = append(holes, holes)
//
// Don't do that. The obvious will happen.
type SecurityHoles []SecurityHole

func (sh SecurityHoles) applySecurityHole(s *security) {
	for _, hole := range sh {
		hole.applySecurityHole(s)
	}
}
