# ODBC Driver for Go - High-Level Overview

## Executive Summary

The ODBC Driver for Go is a production-ready, cross-platform database driver that enables Go applications to connect to any ODBC-compliant database. It implements the standard Go `database/sql` interface, making it a drop-in solution for developers who need SQL database access.

**Key Strengths:**
- ✅ Works with 100+ ODBC-compliant databases (SQL Server, MySQL, Access, FoxPro, etc.)
- ✅ Cross-platform: Windows, Linux, macOS
- ✅ Built-in connection pooling and transaction support
- ✅ Minimal external dependencies (only 2 direct dependencies)
- ✅ Production-tested with multiple database backends

## What Does This Project Do?

### The Problem It Solves

Go applications that need to connect to legacy or enterprise databases often face limited options:
- Some databases don't have native Go drivers
- Legacy systems rely on ODBC as their connectivity standard
- Organizations have existing ODBC infrastructure and configurations
- Different departments use different database systems

### The Solution

This driver acts as a **bridge** between Go's standard database interface and the system's native ODBC layer, allowing any Go application to access any ODBC-compliant database without learning database-specific APIs.

**Before:** Developers had to choose between:
- Writing custom C code to access ODBC
- Using incomplete or outdated Go drivers
- Avoiding Go for projects requiring ODBC access

**After:** Developers can simply:
```go
import _ "github.com/alexbrainman/odbc"

db, _ := sql.Open("odbc", "DSN=MyDatabase")
rows, _ := db.Query("SELECT * FROM users")
```

## How It Works - High Level

### 1. **Connection Flow**
```
Go Application
    ↓
database/sql interface (standard Go)
    ↓
ODBC Driver (this project)
    ↓
System ODBC layer (ODBC32.DLL on Windows, unixODBC on Linux)
    ↓
Database-specific ODBC driver (e.g., SQL Server ODBC driver)
    ↓
Actual Database
```

### 2. **Key Components**

#### Public API Layer
- **Driver**: Registers itself with Go's `database/sql` package
- **Connection**: Manages database connections, including pooling
- **Statement**: Prepares and executes SQL queries
- **Rows**: Iterates through query results
- **Transaction**: Handles multi-statement transactions

#### Internal Implementation
- **ODBC API Bindings**: Low-level calls to the system's ODBC library
- **Parameter Binding**: Converts Go types to ODBC types for parameters
- **Column Mapping**: Converts ODBC types back to Go types for results
- **Error Handling**: Translates ODBC diagnostic records to Go errors

### 3. **Platform Support**

| Platform | ODBC Source | Build Requirement |
|----------|-------------|-------------------|
| Windows 32-bit | odbc32.dll | Visual C++ Build Tools |
| Windows 64-bit | odbc32.dll | Visual C++ Build Tools |
| Linux | unixODBC | `sudo apt-get install unixodbc-dev` |
| macOS | unixODBC | `brew install unixodbc` |

## Supported Databases

Via ODBC, this driver works with:

**Enterprise Systems**
- Microsoft SQL Server (2008, 2012, 2016, 2019, 2022)
- Oracle Database
- IBM DB2
- PostgreSQL (via ODBC driver)

**Business Applications**
- Microsoft Access
- FoxPro
- Paradox
- dBASE

**Open Source**
- MySQL
- MariaDB
- SQLite (with ODBC bridge)

**Cloud Services**
- Azure SQL Database
- Amazon Redshift (via ODBC)
- Google Cloud SQL (via ODBC)

## Quick Start for Developers

### Installation
```bash
go get github.com/alexbrainman/odbc
```

### Basic Usage
```go
package main

import (
    "database/sql"
    _ "github.com/alexbrainman/odbc"
)

func main() {
    // Connect using a DSN (configured in system ODBC settings)
    db, err := sql.Open("odbc", "DSN=MyDatabaseDSN")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Query data
    rows, err := db.Query("SELECT id, name FROM users WHERE active = ?", true)
    if err != nil {
        panic(err)
    }
    defer rows.Close()

    for rows.Next() {
        var id int
        var name string
        rows.Scan(&id, &name)
        println(id, name)
    }
}
```

### Setting Up a Connection

1. **Install ODBC driver** for your database
2. **Configure DSN** in system ODBC settings (or use connection string)
3. **Test connection** with system tools (e.g., `odbctest` on Windows, `isql` on Linux)
4. **Use in Go code** as shown above

## Architecture Overview

### Three-Layer Design

```
┌─────────────────────────────────┐
│    Public Driver Layer          │  <- What users interact with
│  (Conn, Stmt, Rows, Tx)         │     Implements database/sql interfaces
└──────────────┬──────────────────┘
               │
┌──────────────▼──────────────────┐
│    ODBC Implementation Layer     │  <- Internal machinery
│  (ODBCStmt, Params, Columns)    │     Handles type conversion
└──────────────┬──────────────────┘
               │
┌──────────────▼──────────────────┐
│   C API Binding Layer            │  <- Low-level C interop
│  (api package with CGO)          │     Direct ODBC API calls
└─────────────────────────────────┘
```

### Key Design Principles

1. **Separation of Concerns**
   - Each layer has a single responsibility
   - Unsafe C operations isolated in `api` package
   - Easy to test and maintain

2. **Minimal Dependencies**
   - Only 2 direct Go dependencies
   - Relies on system-provided ODBC libraries
   - Reduces version conflicts and deployment issues

3. **Standard Go Patterns**
   - Implements `database/sql/driver` interfaces
   - Follows Go idioms and best practices
   - Works with any Go database abstraction layer (gorm, sqlc, etc.)

## Common Use Cases

### 1. **Legacy System Integration**
Connecting modern Go services to enterprise databases that only expose ODBC drivers.

### 2. **Multi-Database Support**
Building applications that need to support multiple database backends without code changes.

### 3. **Migration Projects**
Gradually migrating from legacy systems to Go while maintaining database compatibility.

### 4. **Enterprise Reporting**
Building reporting tools that need access to various enterprise data sources.

### 5. **System Administration Tools**
Creating administrative tools that interact with database-agnostic ODBC infrastructure.

## Performance Characteristics

### Connection Pooling
- Automatic connection pooling at the driver level
- Configurable max connections and idle timeouts
- Connection validation on reuse

### Query Execution
- Prepared statements support for repeated queries
- Parameter binding with type checking
- Result streaming for large result sets

### Type Conversion
- Efficient Go-to-ODBC type mapping
- NULL value handling
- Support for all standard SQL types (numeric, string, date/time, binary)

## Comparison with Alternatives

| Feature | ODBC Driver | Native Drivers | ORM Libraries |
|---------|-------------|----------------|---------------|
| **Database Support** | 100+ via ODBC | 1-2 per driver | Multiple (depends) |
| **Setup Complexity** | ODBC configuration | Install package | Install + config |
| **Learning Curve** | Steep (ODBC knowledge) | Depends on DB | Moderate (ORM-specific) |
| **Performance** | Very good | Excellent | Good (with overhead) |
| **Legacy Support** | Excellent | Limited | Limited |
| **Type Safety** | Runtime checked | Compile-time (varies) | Compile-time (varies) |

## Troubleshooting Quick Guide

### "DSN not found"
- Verify DSN is configured in system ODBC settings
- On Windows: Use "ODBC Data Source Administrator"
- On Linux: Check `/etc/odbc.ini` or `~/.odbc.ini`

### "Cannot load ODBC driver"
- Ensure ODBC driver for your database is installed
- Check driver name matches ODBC registry/configuration
- Verify driver file is in system driver directory

### "Character encoding issues"
- The driver uses UTF-16 for ODBC, UTF-8 for Go
- Verify database collation supports UTF-8
- Check client character set configuration

### "Connection pooling issues"
- Use `db.SetMaxOpenConns()` to limit concurrent connections
- Use `db.SetMaxIdleConns()` to control idle connection timeout
- Test with `db.Ping()` to verify connection health

## Getting Help

### Documentation
- **Official Wiki**: https://github.com/alexbrainman/odbc/wiki
- **ODBC Standard**: Microsoft ODBC Reference documentation
- **Go database/sql**: https://golang.org/pkg/database/sql/

### Community
- **GitHub Issues**: Report bugs and ask questions
- **Stack Overflow**: Tag with `[go]` and `[odbc]`
- **Project Author**: Alex Brainman (GitHub: @alexbrainman)

## Development Status

### Current Version
- Stable, production-ready
- Active maintenance
- Regular database compatibility testing

### Testing
- Tested with SQL Server, MySQL, Access, FoxPro
- Platform-specific tests for Windows and Unix
- Integration tests with real database connections

### Roadmap Considerations
- Performance optimizations for large datasets
- Enhanced error diagnostics
- Better support for newer ODBC features
- Improved connection string handling

## Summary

The ODBC Driver for Go solves a critical need: **enabling Go applications to work with any ODBC-compliant database through a standard, familiar interface**. It's particularly valuable for:

- Organizations with diverse database infrastructure
- Legacy system integration projects
- Situations where native Go drivers don't exist
- Teams wanting to leverage Go without losing database compatibility

The driver balances complexity (handling 100+ database backends) with simplicity (minimal dependencies, standard Go patterns), making it an excellent choice for production Go applications that need ODBC access.
