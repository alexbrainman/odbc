odbc driver written in go. Implements database driver interface as used by standard database/sql package. It calls into odbc dll on Windows, and uses cgo (unixODBC) everywhere else.

To get started using odbc, have a look at the following pages:
  * [Getting started on Linux](GettingStartedOnLinux.md)
  * [Getting started on OS X](GettingStartedOnOSX.md)
  * [Getting started on Windows](GettingStartedOnWindows.md)