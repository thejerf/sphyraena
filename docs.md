What Is Security?
-----------------

If you really get down to it, just like almost every other term in use in
the software engineering world, "security" is a bit amorphous. The core
definition I tend to use for network software is:

  A system is *secure* when users of the system can perform only and
  exactly the actions the owners of the system desire them to perform.

Read sufficiently broadly, this turns out to cover quite a lot of the
bases. A DOS is a security violation because it prevents users from being
able to do the things the owner wants. An attacker sniffing traffic
violates the owner's desires to share only with certain authorized
people. And so on. It's quite powerful. The question of who the "owner" is
can sometimes get complicated, but it still generally works.

Within that general definition, one can slice and dice in a number of
ways. One useful way that will help you understand Sphyraena is to divide
things between "cryptography" and "application security".

Cryptography is, well, probably exactly what you
expect. AES. Authentication and session security. TLS. HTTPS. All that
jazz that is hard to the point of near impossibility to get right, and that
you should never implement yourself if you can possibly help it.

And in a web framework, you have virtually no choices. You use
HTTPS. You've got some limited control over things like secure cookies and
HSTS. And yes, Sphyraena has first-class support for such things, but in
terms of "web frameworks" that doesn't take much support or work. What
Sphyraena offers could easily be added to other framework.

Where my passion lies with Sphyraena is application security. Who is this
user? Do they have access to this resource? Read or write? How did they
identify this resource? What if somebody else tries to access this
resource? What if someone else tries to attack them via this resource?

Part of Sphyraena's job is to be very opinionated about application
security. We have firm opinions about authentication, such as "I can't
literally stop you from storing plain text passwords but I can make it
easier to do the right thing". It clearly distinguishes between
"authentication" and "authorization", and actually ships with a default,
useful model for authorization. It defaults to the most secure defaults it
can, whenever it can; you don't have to turn *on* frame hijacking
prevention, you have to turn it *off* if for some reason you need to allow
it. Content Security Policies, a thing many web developers don't even know
exist, default to highly secure values that you must tune back down. As I
like to say, 9.5 out of 10 of the top 10 OWASP vulnerabilities can be
mitigated by Sphyraena. (I dock Sphyraena .5 just because there's a
difference between _providing_ a useful authorization model and your
application _using_ it. But still, it's ___much___ better than nothing!)

I look around the world of web frameworks, and virtually nothing out there
has this focus. Most frameworks consider their security task to end when
they hand you the user from the session object. Many frameworks have
absolutely no concept of restricting access to a resource. Even fewer still
are designed with making it easy to restrict access; for instance, witness
Sphyraena's slightly different approach to routing... when viewed as a
security measure against attackers even being able to _spell_ access to
resources they shouldn't have access to, it begins to make sense. (Plus the
fact that from a software engineering perspective it's just nasty to hard
code the router to handlers like that without an intermediate object.)

Namespacing as a Security Measure

Namespacing is an important and easily-overlooked aspect of security. A
namespace is a mapping of some naming scheme of symbols to some set of
resources. It is what determines what nouns a given piece of code may
"refer to" when making requests. An entire class of security vulnerabilities
amount to failures to either properly define or properly secure a bit of
code's namespace.

Consider the common security violation that any user can simply enumerate
all records by incrementing a readily-visible ID in the URL. You can look
at this as a failure to check permissions on the records, and deny access
for users who should not have been permitted. This is not wrong, inasmuch
as this is always something that you should be doing. But an
even better way of looking at the problem is that the namespace of the
code was too permissive, and it *allowed* remote users to ask for things
that weren't theirs. If you make it so that a remote user can not even
*talk* about records that don't belong to them, if you make it so no
combination of bytes sent to you can refer to somebody else's record, then
a hacker's job becomes much harder. And of course, defense-in-depth
means that you should *still* check the permissions even so. The
combination of the two is far more difficult to overcome than either
alone.

In the simplest case, one can modify the database access such that the
record is keyed by (CURRENT_LOGGED_IN_USER, database ID), making it
so that the current logged in user is taken directly from the session,
where the user can not touch it. This will make it so that at the very
least, a database query for other records will fail. Slightly better
would be to add a layer to the querystring such that even the database
ID is no longer visible to the user, but only some opaque token that
the framework translates back to an ID for you, using $FEATURE.
