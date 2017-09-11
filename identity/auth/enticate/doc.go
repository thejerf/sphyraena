/*

Package enticate handles authentication skeletons and base implementations
for authentications.

This package defines an interface that all Sphyraena-conforming
authentication objects must conform to, and provides some simple default
authentication techniques. Additional authentication techniques may be
plugged in by conforming to the given interfaces.

Extending Authentication Objects

This is not a feature of Sphyraena, but rather Go's interfaces, but it
bears calling out. There are many ways you may wish to provide a richer
authentication object. This can trivially be done by returning a rich
object that conforms to the Authentication interface, which you can
then cast in your own code to the richer interface. Sphyraena's interface
is deliberately minimalistic to constrain your own implementations as
little as possible.

*/
package enticate
