# buckets 

[![GoDoc](https://godoc.org/github.com/joyrexus/buckets?status.svg)](https://godoc.org/github.com/joyrexus/buckets)

A simple key/value store based on [Bolt](https://github.com/boltdb/bolt). 

![buckets](buckets.jpg)

In the parlance of [key/value stores](https://en.wikipedia.org/wiki/Key-value_database), a "bucket" is a collection of unique keys that are associated with values. A buckets database is a set of buckets.  The underlying datastore is represented by a single file on disk.  

Note that buckets is just an extension of Bolt, providing a `Bucket` type with some nifty convenience methods for operating on the key/value pairs within instances of it.

Use `go get github.com/joyrexus/buckets` to install and see the [docs](https://godoc.org/github.com/joyrexus/buckets) for details.