/*

Package elements contains complete implementations of various elements
of sphyraena, such as router clauses, handlers, etc.

This is sort of also the Sphyraena "samples" directory, but I didn't
want to give this directory a name that implies it isn't suitable for
real usage. With the possible exception of the "sites", this is intended
to be production-quality stuff in here, and patches to make these
elements even more-so production quality will be accepted.

This package itself doesn't have anything in it. Categories of various
elements are available in the subdirectories. Small elements will be
in those directories, usually in one file. Larger components may get
their own module.

The exception is the "sites" directory, which contains modules that are
intended to be full sites, and as a result, contain full executables in
them.

Each of these elements is intended to be production quality on its own
merits; however, it is also intended that if you need to clone & modify
them, you can. None of these components could possibly meet all needs.

These components also serve as samples of how to do the things they
demonstrate.

*/
package elements
