Sphyraena
=========

Status: Utterly experimental, with no promise whatsoever that it will ever
move out of that phase.

An experimental web framework for Go. The two-and-a-half raison d'etre are:

* Default secure. Where ever possible, the web is turned into a default
  secure environment rather than a default insecure environment that must
  laboriously have all of its security switches flipped back on.

  This also means the framework requires that all users be at all times
  authenticated and authorized, and (eventually) provide evidence that some
  sort of security check was performed.

* Default streaming: Rather than the web being viewed as a series of static
  pages, this framework *defaults* to seeing the world as a series of
  composable streams, for which static web pages are merely one
  particularly interesting case.

  I get the sense that some web frameworks are headed this way, but it
  still seems to come with more baggage than I'd like. This is designed to
  make streaming as easy as communicating over Go channels.

* And this is the "half" - this framework is designed to be used in places
  where people care about auditing, and thus, designed to be highly
  auditable. In particular, some effort is taken to make the routing tables
  comprehensible to auditors.

  The routing is also heavily, heavily biased in the direction of providing
  power, rather than speed. The idea is that a Sphyraena application could
  replace arbitrarily complicated nginx configurations eventually. To do
  that, framework users need to be able to provide their own routing
  clauses.

At the moment this is only up on GitHub for personal convenience. I expect
it would be outright impossible for anyone other than me to bring a
Sphyraena website up and actually use the features described above, not to
mention some of them don't exist yet.

