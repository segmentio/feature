# feature ![build status](https://github.com/segmentio/feature/actions/workflows/test.yml/badge.svg)
Feature gate database designed for simplicity and efficiency.

## Motivation

Feature gates are an important part of controlling the risk associated with
software releases, they bring safe guards and granular knobs over the exposure
of data to new code paths.

However, these promises can only be kept if programs can reliably access the
feature gate data, and efficiently query the data set. Most feature gate systems
rely on performing network calls to a foreign system, creating opportunities for
cascading failures in distributed systems where feature gate checks are often
performed on critical data paths.

The `feature` package was designed to offer high availbility of the feature
gates, and high query performance, allowing its use in large scale systems with
many *nines* of uptime like those we run at Segment.

### Reliability

The feature database is represented by an immutable set of directories and
files on a file system. The level of reliability offered by a set of files on
disk exceeds by a wide margin what we can achieve with a daemon process serving
the data over a network interface. Would the program updating the feature
database be restarted or crashed, the files would remain available for consumers
to read and query. The system is known to _"fail static"_: in the worst case
scenario, nothing changes.

### Efficiency

The feature database being immutable, it enables highly efficient access to
the data. Programs can implement simple and high performance caching layers
because they don't need to manage cache expirations or transactional updates.
The database files are mapped to read-only memory areas and therefore can be
shared by all collocated processes, ensuring that a single copy of the data
ever exists in memory.

## Data Models

### Collections

Collections are lists of unique identifiers that programs can query the state
of gates for (either open and closed). The collections are arranged in groups
and tiers. Each group may have multiple tiers, within each tier the collection
files contain the list of identifiers (one by line).

Here is an example of the on-disk representation of collections:
```
$ tree
.
└── standard
    ├── 1
    │   ├── collections
    │   │   ├── source
    │   │   ├── workspace
    │   │   └── write_key
...
```
_For the standard group, tier 1, there are three collections of source, workspace
and write keys._

```
$ cat ./standard/1/collections/source
ACAtsprztv
B458ru47n7
CQRxBaQSt8
EJw9i04Lsv
IbQor7hHBU
LZK0HYwDTH
MKOxgJsedB
OmNMfU6RbP
Q5lmdTzq1Y
SqNT0bDYl7
...
```

On-disk file structures with large number of directories and small files cause
space usage amplification, leading to large amounts of wasted space.
By analyzing the volume of data used to represent feature flags, we observed
that most of the space was used by the collections of identifiers. Text files
provide a compact representation of the identifiers, minimizing storage space
waste caused by block size alignment, and offering a scalable model to grow
the number of collections and identifiers in the system.

### Gates

The second core data type are feature gates, which are grouped by family, name,
and collections that they apply to. The gate family and name are represented as
directories, and the gate data per collection are stored in text files of
key/value pairs.

Continuing on our previous example, here is a view of the way gates are laid out
in the file system:
```
$ tree
.
└── standard
    ├── 1
...
    │   └── gates
    │       ├── access-management
    │       │   └── invite-flow-enabled
    │       │       └── workspace
...
```
_For the standard group, tier 1, gate invite-flow-enabled of the
access-management family is enabled for workspaces._

```
$ cat ./standard/1/gates/access-management/invite-flow-enabled/workspace
open	true
salt	3653824901
volume	1
```

The gate files contain key value pairs for the few properties of a gate,
which determine which of the identifiers will see the gate open or closed.

| Key    | Value                                                                                              |
| ------ | -------------------------------------------------------------------------------------------------- |
| open   | true/false, indicates the default behavior for identifiers that aren't in the collection file      |
| salt   | random value injected in the hash function used to determine the gate open state                   |
| volume | floating point number between 0 and 1 defining the volume of identifiers that the gate is open for |

## Using the CLI

The `cmd/feature` program can be used to explore the state of a feature
database. The CLI has multiple subcommands, we present a few useful ones
in this section.

All subcommand understand the following options:

| Option         | Environment Variable | Description                                     |
| -------------- | -------------------- | ----------------------------------------------- |
| `-p`, `--path` | `FEATURE_PATH`       | Path to the feature database to run commands on |

The `FEATURE_PATH` environment variable provides a simple mechanism to configure
configure the _default_ database used by the command:

```shell
$ export FEATURE_PATH=/path/to/features
```

By default, the `$HOME/feature` directory is used.

### `feature get gates [collection] [id]`

This command prints the list of gates enabled for an identifier, it is useful
to determine whether a gate is open for a given id, for example:

```
# empty output if the gate is not open
$ feature get gates source B458ru47n7 | grep gate-family | grep gate-name
```

### `feature get tiers`

This command prints a summary of the tiers that exist in the feature database,
here is an example:

```
$ feature get tiers
GROUP      TIER  COLLECTIONS  FAMILIES  GATES
standard   7     0            17        39
standard   6     0            18        40
standard   1     3            20        109
standard   8     0            17        39
standard   4     3            18        41
standard   3     0            18        41
standard   2     3            19        107
standard   5     3            18        40
```

### `feature describe collection [-g group] [-t tier] [collection]`

This command prints the list of identifiers in a collection, optinally filtering
on a group and tier (by default all groups and tiers are shown).

```
$ feature describe collection workspace
96x782dXhZmn6RpPJVDXgG
4o74gqFGmTgq7GS6EN3ZQJ
mcYdYvfZQcUaid1CVdC9F3
nRRroPD8pV3giaetjpDmu7
96x782dXhZmn6RpPJVDXgG
1232rt203
9a2aceada5
cus_HbXktPfAbH3weZ
opzvxHK692ZJJicNxz1AfL
pkpdcdSLNX14Za6qpD7wtv
...
```

_Note: the identifiers are not displayed in any particular order, this command
simply iterate over the directories and scans the collection files._

### `feature describe tier [group] [tier]`

This command shows a verbose description of a tier, including the list of
collections, and the state of each gate in the tier:

```
$ feature describe tier standard 1
Group:	standard
Tier:	1

Collections:
 - write_key
 - workspace
 - source

Gates:
  integrations-consumer/observability-discards-gate
  - workspace	(100%, default: open)

  destinations-59ceac7c2828a60001d22936/centrifuge_qa
  - workspace	(100%, default: open)

  destinations-54521fdc25e721e32a72ef04/webhook-flagon-centrifuge
  - write_key	(100%, default: close)

...
```

## Using the Go API

The `feature` package provides APIs to consume the feature gate data set, this
section presents on the most common use cases that programs have and how they
are solved by the package.

```go
import (
    "github.com/segmentio/feature"
)
```

### `feature.MountPoint`

The `feature.MountPoint` type represents a path on the file system where a
feature database is mounted. This type is the entry point to all other APIs,
typically a program will construct one mount point from a configuration option
or environment variable:

```go
mountPoint := feature.MountPoint("/path/to/features")
```

_Note: prefer using an absolute path for the mount point, so operations are
not dependent on the working directory._

### `feature.Store`

From a mount point, a program can open a feature database, which is materialized
by a `feature.Store` object.

```go
features, err := mountPoint.Open()
if err != nil {
    fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
} else {
    ...
}
```

The `feature.Store` type will watch for changes to the mount point, and
automatically reload the content of the feature database when a change is
detected. This mechanism assumes that the feature database is immutable,
programs that intend to apply updates to the database must recreate it and
replace the entire directory structure (which should be done in an atomic
fashion via the use of the `rename(2)` syscall for example).

### `feature.(*Store).OpenGate`

This is the most common use case for programs, the `OpenGate` method tests
whether a gate is open for a given identifier.

The gate is defined by the pair of gate family and name, while the identifier
is expressed as a pair of the collection and its value.

```go
if features.OpenGate("gate-family", "gate-name", "collection", "1234") {
    ...
}
```

### `feature.(*Store).LookupGates`

Another common use case is for programs to lookup the list of gates that are
enabled on an identifier. The `LookupGates` method solves for this use case.

```go
for _, gate := range features.LookupGates("gate-family", "collection", "1234") {
    ...
}
```

_Note: the `feature.Store` type uses an internal cache to optimize gate lookups,
programs must treat the returned slice as an immutable value to avoid race
conditions. If the slice needs to be modified, a copy must be made first._
