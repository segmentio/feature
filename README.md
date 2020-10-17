# feature [![Build status](https://badge.buildkite.com/241bc64456fcbe3cc363c75ade4f473ad92af6284cb68858db.svg)](https://buildkite.com/segment/feature?branch=main)
Experimental feature gate database designed with simplicity and efficiency in mind.

## Motivation

Feature gates are an important part of releasing software changes, they bring
safe guards and granular control over the exposure of data to new code paths.
However, these promises can only be kept if programs can reliably access the
feature gate data, and efficiently query the data set.

The `feature` package was designed with these two goals in mind, offering a
feature gate model compatible with flagon, but designed to remove the need for
running a sidecar to access the feature gate database.

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

The `feature` package being compatible with [`flagon`](https://github.com/segmentio/flagon),
its data models are mostly one-to-one mappings with flagon concepts.

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
