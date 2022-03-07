# Introduction

_this repository is a fork of [alexbrainman/odbc](https://github.com/alexbrainman/odbc) and all
credits go to the author(s) of this package_

## The reason for the fork

We have troubles with the Spark `string` type and the `alexbrainman/odbc` package. We have created
an [issue](https://github.com/alexbrainman/odbc/issues/165) in the original repository explaining
our issues in detail.

In this fork, we modify some of the column binding operations to work more nicely with Spark.

We also implement the [`driver.QueryerContext`](https://pkg.go.dev/database/sql/driver#QueryerContext)
which honours the context passed in, and returns when the context times out or gets cancelled.

## Original `README.md`

odbc driver written in go. Implements database driver interface as used by standard database/sql package. It calls into odbc dll on Windows, and uses cgo (unixODBC) everywhere else.

To get started using odbc, have a look at the [wiki](../../wiki) pages.
