# anet - AtmanOS networking

anet implements the Xen network interface for low-level device interactions on
AtmanOS, with an adapter for using these devices with [netstack].

  [netstack]: https://github.com/google/netstack

## API Stability

This package combines types extracted from AtmanOS and early networking demos.
The (lack of) package boundaries are not intentional and will likely revised in
the future. In particular: there's no separation here between Xen, netstack,
or common bits; also, what's exported or not is mostly historical accident.
