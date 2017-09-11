/*

Package auth describes its subpackages.

One of the classic mistakes someone can make in security is to conflate
"authentication" with "authorization". The similarity of the words can't
help, and neither does the fact that authentication can be used as a
degenerate form of authorization, making it even easier to be confused.
Those frameworks out there that reify the error make it even easier.

"Authentication" is establishing that a given entity is indeed the entity
it claims to be. Often this is a person, but it can also be a computer
or something else.

In particular, a given entity really ought to have *one* authentication,
ever. Bob Howard is always Bob Howard. Even if he has several capacities
he may be acting under, and even if he is acting directly on someone
else's behalf, he is still Bob Howard, and the system should never lose
track of this fact.

"Authorization" is a set of actions that an entity is allowed to take.
For instance, an entity may be allowed to "create new accounts" or "view
the budget for the IT department". Note an authorization does not
reference any entity, they simply exist.

These two words are in common usage. Many well-engineered systems discover
a third concept that does not have a universal name that I know of.
In database terms, we'd describe this concept as a many-to-many relationship
between an "authentication" and "authorizations". Consider the common
case of a user who can log in as an administrator, and view the system
from the point of view of another user, perhaps for support reasons.
In this case, the administrator's authentication never changes, but they
are using an authorization that "belongs" to another user.

Many systems also do something useful where a user with very rich
authorizations can voluntarily switch down to a lower set, not so much
for security reasons, but simply to simplify the UI. For instance,
someone who can administrate a thousand domains may wish to "narrow
down" the system so they are administrating just one.

This lacks a common name, so for various historical reasons stemming from
Sphyraena's origin point, we will call the combination of an Authentication
and an Authorization a Role.

Naming Convention

In the interests of keeping the two concepts as separate as possible,
I factored out the "auth", and the two modules for handling these
two concepts are now "enticate" and "orization".

Sphyraena's Auth Support

Sphyraena, as a security-focused framework, insists that all web requests
are accompanied by an authentication and an authorization. As much code
as possible is provided to make this as easy as possible, while still being
expressed as much as possible in terms of interfaces that permit any
arbitrary swapping-out that you may need.

*/
package auth
