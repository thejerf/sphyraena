/*

Package context manages Spyhraena's context object.

As with many other frameworks, Sphyraena provides a context
object. In order to conform with Sphyraena's philosophy and goals,
it is rather more intrusive than you may be used to, because direct,
unmediated access to the request object makes too easy a large number
of bad security decisions.

One of the simplest examples of this unabashed security paternalism
can be seen in the method used to obtain a request's Referer. It's
too easy to write code that gets the referer (using the now-traditional
misspelling) and then does something with it that is, statistically
speaking, a bad idea. Thus, this requires you to call a method called
UntrustedReferer() rather than just Referer(), so that (ideally) a
programmer calling this method will think twice about doing something
based on it, and failing that, in a review, a reviewer will be
reminded by the source code itself that this is not the safest method
to call.

*/
package context
