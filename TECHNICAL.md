# ODBC Driver for Go - Technical Documentation

## Table of Contents

1. [Architecture Deep Dive](#architecture-deep-dive)
2. [API Reference](#api-reference)
3. [Type System](#type-system)
4. [Implementation Details](#implementation-details)
5. [Error Handling](#error-handling)
6. [Performance Optimization](#performance-optimization)
7. [Platform-Specific Notes](#platform-specific-notes)
8. [Connection String Format](#connection-string-format)
9. [Advanced Usage](#advanced-usage)
10. [Internal Development](#internal-development)

---

## Architecture Deep Dive

### Module Structure

```
github.com/alexbrainman/odbc/
├── api/              # CGO bindings to ODBC C libraries
│   ├── api.go       # Shared type definitions
│   ├── api_windows.go
│   ├── api_windows_386.go
│   ├── api_windows_amd64.go
│   ├── api_unix.go
│   ├── zapi_windows.go    # GENERATED - DO NOT EDIT
│   └── zapi_unix.go       # GENERATED - DO NOT EDIT
├── driver.go        # Driver registration and initialization
├── conn.go          # Connection lifecycle (public API)
├── stmt.go          # Statement wrapper (public API)
├── odbcstmt.go      # ODBC statement implementation
├── rows.go          # Result iteration (public API)
├── result.go        # Execution results
├── tx.go            # Transaction handling
├── param.go         # Parameter binding and type conversion
├── column.go        # Column metadata and result binding
├── error.go         # Error handling and diagnostics
├── handle.go        # ODBC handle management
├── utf16.go         # UTF-16 encoding utilities
└── stats.go         # Connection statistics
```

### Initialization Flow

```
1. init() in driver.go
   ├─ initDriver()
   │  ├─ SQLAllocHandle(SQL_HANDLE_ENV)  -> environment handle
   │  ├─ SQLSetEnvUIntPtrAttr(ODBC_VERSION = 3)
   │  ├─ SQLSetEnvUIntPtrAttr(CONNECTION_POOLING = ONE_PER_HENV)
   │  └─ SQLSetEnvUIntPtrAttr(CP_MATCH = RELAXED)
   └─ sql.Register("odbc", &drv)

2. User calls sql.Open("odbc", dsn)
   ├─ driver.Open() is called
   ├─ SQLAllocHandle(SQL_HANDLE_DBC, environment_handle)
   ├─ SQLDriverConnect() with DSN/connection string
   └─ Returns *Conn

3. User calls db.Query() or db.Exec()
   ├─ conn.Prepare() or conn.PrepareContext()
   ├─ conn.PrepareODBCStmt() creates ODBCStmt
   ├─ ODBCStmt.execute() or ODBCStmt.fetch()
   └─ Results returned via *Rows
```

### Handle Hierarchy

```
SQLHENV (Environment Handle)
│
├─ Created in: initDriver()
├─ Attributes: ODBC version, connection pooling
└─ Lifecycle: One per process, never explicitly freed
    (TODO comment in code suggests this is a design issue)

    ↓

SQLHDBC (Connection Handle)
│
├─ Created in: Driver.Open()
├─ Parent: Environment handle
├─ Attributes: Connection timeout, transaction isolation
└─ Lifecycle: One per database connection

    ↓

SQLHSTMT (Statement Handle)
│
├─ Created in: ODBCStmt.execute()
├─ Parent: Connection handle
├─ Attributes: Query timeout, cursor behavior
└─ Lifecycle: One per prepared statement
```

### Object Hierarchy

```
*Conn (implements driver.Conn)
├─ h (SQLHDBC)           - Connection handle
├─ tx (*Tx)              - Current transaction if active
├─ bad (bool)            - Connection health status
└─ isMSAccessDriver (bool) - Database-specific flag

    ↓ Conn.Prepare()

*Stmt (implements driver.Stmt)
├─ c (*Conn)             - Parent connection
├─ query (string)        - SQL query string
├─ os (*ODBCStmt)        - Internal ODBC statement
└─ mu (sync.Mutex)       - Thread-safe access

    ↓ Stmt.Query() or Stmt.Exec()

*ODBCStmt (internal implementation)
├─ h (SQLHSTMT)          - Statement handle
├─ Parameters ([]Parameter) - Prepared parameters
├─ cols ([]column)       - Result columns
├─ boundCols ([]bool)    - Binding status
└─ statementHandle       - Parent connection reference

    ↓ ODBCStmt.fetch()

*Rows (implements driver.Rows)
├─ os (*ODBCStmt)        - Statement for fetching
└─ closed (bool)         - Lifecycle flag
```

---

## API Reference

### Public API (database/sql/driver interfaces)

#### Driver Interface

```go
type Driver interface {
    Open(name string) (Conn, error)
}

// Implementation in driver.go
func (d *Driver) Open(dsn string) (driver.Conn, error)
```

**Parameters:**
- `dsn` (string): ODBC data source name or connection string

**Returns:**
- `driver.Conn`: Connected database connection
- `error`: Connection error if unable to connect

**Examples:**
```go
// Using DSN configured in system ODBC settings
"DSN=MyDatabase"

// Using connection string with credentials
"Driver={ODBC Driver 17 for SQL Server};Server=localhost;Database=mydb;UID=sa;PWD=password"

// Access with relative path
"Driver={Microsoft Access Driver (*.mdb, *.accdb)};DBQ=C:\\path\\to\\database.mdb"
```

#### Conn Interface

```go
type Conn interface {
    Prepare(query string) (Stmt, error)
    Close() error
    Begin() (Tx, error)
}

// Additional methods
func (c *Conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error)
func (c *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error)
func (c *Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error)
func (c *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error)
```

#### Stmt Interface

```go
type Stmt interface {
    Close() error
    NumInput() int
    Exec(args []driver.Value) (Result, error)
    Query(args []driver.Value) (Rows, error)
}

// Additional methods
func (s *Stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error)
func (s *Stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error)
```

#### Rows Interface

```go
type Rows interface {
    Columns() []string
    Close() error
    Next(dest []driver.Value) error
}
```

#### Result Interface

```go
type Result interface {
    LastInsertId() (int64, error)
    RowsAffected() (int64, error)
}
```

### Internal API

#### ODBCStmt Methods

```go
type ODBCStmt struct {
    h            api.SQLHSTMT
    Parameters   []Parameter
    cols         []column
    boundCols    []bool
}

// Core methods
func (os *ODBCStmt) execute(args []driver.NamedValue) (int64, error)
func (os *ODBCStmt) fetch(dest []driver.Value) error
func (os *ODBCStmt) moreResults() (bool, error)
func (os *ODBCStmt) closeByStmt() error
```

#### Parameter Handling

```go
type Parameter struct {
    BindType    api.SQLSMALLINT
    ValueType   api.SQLSMALLINT
    ColumnSize  api.SQLULEN
    DecimalPlaces api.SQLSMALLINT
    Value       interface{}
    StrValue    string
    BinaryValue []byte
}

// Internal helper
func bindParam(p *Parameter, paramIndex int) error
```

#### Column Handling

```go
type column struct {
    Name         string
    Type         api.SQLSMALLINT
    Size         api.SQLULEN
    DecimalDigits api.SQLSMALLINT
    Nullable     bool
    BoundData    interface{}
    Buffer       []byte
}

// Internal helper
func bindCol(c *column, colIndex int) error
```

---

## Type System

### ODBC Type Mapping

The driver automatically converts between Go types and ODBC types:

#### Numeric Types

| Go Type | ODBC Type | Notes |
|---------|-----------|-------|
| `int`, `int32` | SQL_INTEGER | 32-bit signed integer |
| `int64` | SQL_BIGINT | 64-bit signed integer |
| `int16` | SQL_SMALLINT | 16-bit signed integer |
| `uint8` | SQL_TINYINT | 8-bit unsigned |
| `float32` | SQL_REAL | Single precision |
| `float64` | SQL_DOUBLE | Double precision |

#### Decimal/Numeric Types

| Go Type | ODBC Type | Conversion |
|---------|-----------|-----------|
| `string` + decimal | SQL_DECIMAL | Via string parsing |
| `string` + numeric | SQL_NUMERIC | Via string parsing |

#### String Types

| Go Type | ODBC Type | Encoding |
|---------|-----------|----------|
| `string` | SQL_VARCHAR | UTF-16 for ODBC, UTF-8 in Go |
| `string` | SQL_WVARCHAR | Wide characters (UTF-16) |
| `string` | SQL_LONGVARCHAR | Long strings (> 4KB) |

#### Date/Time Types

| Go Type | ODBC Type | Format |
|---------|-----------|--------|
| `time.Time` | SQL_DATE | YYYY-MM-DD |
| `time.Time` | SQL_TIME | HH:MM:SS |
| `time.Time` | SQL_TIMESTAMP | YYYY-MM-DD HH:MM:SS.fff |
| `time.Time` | SQL_SS_TIME2 | SQL Server specific |

#### Binary Types

| Go Type | ODBC Type | Usage |
|---------|-----------|-------|
| `[]byte` | SQL_BINARY | Fixed-length binary |
| `[]byte` | SQL_VARBINARY | Variable-length binary |
| `[]byte` | SQL_LONGVARBINARY | Large binary objects |

#### Special Types

| Go Type | ODBC Type | Notes |
|---------|-----------|-------|
| `nil` | SQL_NULL_DATA | NULL values |
| `bool` | SQL_BIT | Via integer (0/1) conversion |

### Type Conversion Examples

```go
// Integer
var id int
rows.Scan(&id)  // ODBC converts SQL_INTEGER to Go int

// String
var name string
rows.Scan(&name)  // ODBC converts SQL_VARCHAR to Go string

// Date
var created time.Time
rows.Scan(&created)  // ODBC converts SQL_TIMESTAMP to time.Time

// NULL handling
var optional sql.NullString
rows.Scan(&optional)  // ODBC detects NULL and sets Valid=false

// Decimal (use string for precision)
var amount string
rows.Scan(&amount)  // Retrieved as string, parse as decimal.Decimal
```

---

## Implementation Details

### Connection Pooling

The driver enables connection pooling at the environment level:

```go
// In driver.go initDriver()
SQLSetEnvUIntPtrAttr(env, SQL_ATTR_CONNECTION_POOLING, SQL_CP_ONE_PER_HENV, SQL_IS_UINTEGER)
SQLSetEnvUIntPtrAttr(env, SQL_ATTR_CP_MATCH, SQL_CP_RELAXED_MATCH, SQL_IS_UINTEGER)
```

**What this means:**
- ODBC maintains a pool of connections per environment
- Relaxed matching allows reusing connections with slightly different attributes
- Go's `database/sql` package manages connection limits via `SetMaxOpenConns()` and `SetMaxIdleConns()`

**Configuration:**
```go
db, _ := sql.Open("odbc", dsn)
db.SetMaxOpenConns(25)      // Max concurrent connections
db.SetMaxIdleConns(5)       // Max idle connections
db.SetConnMaxLifetime(time.Hour) // Connection lifetime
```

### Parameter Binding

When executing prepared statements with parameters:

```go
// User code
db.QueryRow("SELECT * FROM users WHERE id = ?", 42)

// Driver implementation in param.go
1. Convert Go value to ODBC type
2. Call SQLBindParameter() for each parameter
3. Call SQLExecute()
4. ODBC substitutes parameters in query
5. Database receives complete query
```

**Process:**
```
Input: value = 42 (Go int)
   ↓
Detect type: int -> SQL_C_LONG (C type)
   ↓
Determine SQL type: SQL_INTEGER
   ↓
Call SQLBindParameter(param_index, SQL_PARAM_INPUT, SQL_C_LONG, SQL_INTEGER, ...)
   ↓
ODBC binds parameter
   ↓
SQLExecute() sends complete query to database
```

### Result Binding

When fetching query results:

```go
// User code
rows, _ := db.Query("SELECT id, name FROM users")
for rows.Next() {
    rows.Scan(&id, &name)
}

// Driver implementation in column.go
1. SQLNumResultCols() gets column count
2. SQLDescribeCol() gets column metadata for each column
3. SQLBindCol() binds output buffers
4. SQLFetch() retrieves rows
5. Convert ODBC types to Go values
```

**Process:**
```
Metadata retrieval:
   SQLNumResultCols() -> 2 columns
   ↓
   SQLDescribeCol(1) -> name="id", type=SQL_INTEGER, size=4
   SQLDescribeCol(2) -> name="name", type=SQL_VARCHAR, size=100
   ↓
   SQLBindCol(1, buffer1, SQL_INTEGER)
   SQLBindCol(2, buffer2, SQL_VARCHAR)

Data retrieval per row:
   SQLFetch()
   ↓
   buffer1 = 42 (from database)
   buffer2 = "John Doe" (from database)
   ↓
   Convert: int(buffer1), string(buffer2)
   ↓
   rows.Scan(&id, &name)
```

### Transaction Management

Transactions use ODBC 3.0 auto-commit mode switching:

```go
// User code
tx, _ := db.Begin()
tx.Exec("INSERT INTO ...")
tx.Commit()

// Driver implementation in tx.go
Begin():
   ├─ SQLSetConnectAttr(SQL_ATTR_AUTOCOMMIT, FALSE)
   └─ Returns *Tx

Commit():
   ├─ SQLEndTran(SQL_HANDLE_DBC, handle, SQL_COMMIT)
   └─ SQLSetConnectAttr(SQL_ATTR_AUTOCOMMIT, TRUE)

Rollback():
   ├─ SQLEndTran(SQL_HANDLE_DBC, handle, SQL_ROLLBACK)
   └─ SQLSetConnectAttr(SQL_ATTR_AUTOCOMMIT, TRUE)
```

---

## Error Handling

### Error Structure

```go
type Error struct {
    APIName string         // e.g., "SQLDriverConnect"
    Diag    []DiagRecord   // Diagnostic records from ODBC
}

type DiagRecord struct {
    State       string  // e.g., "08001" (Cannot connect)
    NativeError int     // Database-specific error code
    Message     string  // Human-readable error message
}
```

### Error Retrieval

When an ODBC API call fails:

```go
1. Check return code (SQLRETURN)
   ├─ SQL_SUCCESS (0)
   ├─ SQL_SUCCESS_WITH_INFO (1)
   ├─ SQL_ERROR (-1)
   ├─ SQL_INVALID_HANDLE (-2)
   └─ SQL_NO_DATA (100)

2. If IsError(ret) == true:
   ├─ Call SQLGetDiagRec() in loop
   ├─ Extract diagnostic records
   ├─ Format as Error struct
   └─ Return to caller

3. Check special state codes:
   ├─ "08S01" -> return driver.ErrBadConn
   └─ Others -> return formatted Error
```

### Common State Codes

| Code | Meaning | Typically |
|------|---------|-----------|
| 08001 | Cannot connect | Network unreachable |
| 08003 | Connection not open | Connection closed |
| 08S01 | Communication link failure | Connection lost |
| 22003 | Numeric value out of range | Data overflow |
| 22008 | Datetime field overflow | Invalid date |
| 23000 | Integrity constraint violation | Duplicate key |
| 42S00 | Base table or view not found | Table doesn't exist |
| 42S22 | Column not found | Invalid column name |

### Error Handling Example

```go
rows, err := db.Query("SELECT * FROM nonexistent_table")
if err != nil {
    // err is of type *odbc.Error
    switch err := err.(type) {
    case *odbc.Error:
        for _, diag := range err.Diag {
            fmt.Printf("[%s] %s\n", diag.State, diag.Message)
            // Output: [42S02] [Microsoft][ODBC SQL Server Driver][SQL Server]
            //         Invalid object name 'nonexistent_table'.
        }
    }
}
```

---

## Performance Optimization

### Connection Pooling Tuning

```go
// Default settings (reasonable for most applications)
db.SetMaxOpenConns(25)      // RECOMMENDED: 2-4x CPU cores
db.SetMaxIdleConns(5)       // RECOMMENDED: 1-2x CPU cores
db.SetConnMaxLifetime(0)    // 0 = unlimited, set if issues

// For high-concurrency applications
db.SetMaxOpenConns(100)     // More concurrent queries
db.SetMaxIdleConns(10)      // More idle pools

// For long-running queries
db.SetConnMaxLifetime(30 * time.Minute)  // Refresh connections

// For connection-heavy workloads
db.SetMaxIdleConns(0)       // Close idle connections immediately
```

### Query Optimization

```go
// AVOID: Unprepared statements (parsed each time)
db.QueryRow("SELECT * FROM users WHERE id = ?", 42)

// PREFER: Prepared statements (parsed once, reused)
stmt, _ := db.Prepare("SELECT * FROM users WHERE id = ?")
defer stmt.Close()
stmt.QueryRow(42)
```

### Batch Operations

```go
// AVOID: Individual inserts (slow)
for i := 0; i < 1000; i++ {
    db.Exec("INSERT INTO items VALUES (?, ?)", i, data[i])
}

// PREFER: Transaction batch (faster)
tx, _ := db.Begin()
stmt, _ := tx.Prepare("INSERT INTO items VALUES (?, ?)")
for i := 0; i < 1000; i++ {
    stmt.Exec(i, data[i])
}
stmt.Close()
tx.Commit()

// BEST: Native database BULK INSERT (database-specific)
// Use LOAD DATA INFILE (MySQL), BULK INSERT (SQL Server), etc.
```

### Memory Management

```go
// IMPORTANT: Always close resources
rows, _ := db.Query(...)
defer rows.Close()  // Close result set

stmt, _ := db.Prepare(...)
defer stmt.Close()  // Close prepared statement

conn, _ := db.Conn(context.Background())
defer conn.Close()  // Close explicit connection

// Limit memory for large result sets
// Use pagination instead of loading all rows
db.Query("SELECT * FROM huge_table LIMIT 1000 OFFSET 0")
```

### Connection Validation

```go
// Periodically validate connection health
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        if err := db.Ping(); err != nil {
            log.Printf("Connection lost: %v", err)
            // Handle reconnection
        }
    }
}()
```

---

## Platform-Specific Notes

### Windows Implementation

#### ODBC32.DLL Direct Syscalls

```go
// File: api/api_windows.go
// Uses mksyscall_windows.go to generate syscall stubs

//sys SQLAllocHandle(handleType SQLSMALLINT, ...) ... = odbc32.SQLAllocHandle
```

**Architecture Variants:**
- `api_windows_386.go`: 32-bit constants (pointer size 4)
- `api_windows_amd64.go`: 64-bit constants (pointer size 8)

**Build Command:**
```bash
# Automatically handled by CGO
go build ./...

# Requires: Visual C++ Build Tools (or similar C compiler)
```

**Driver Installation:**
- Control Panel → Administrative Tools → ODBC Data Source Administrator
- Or use command-line: `odbcconf` utility

#### Testing on Windows

```bash
# SQL Server test
go test -v -run TestMssql ./

# Access test
go test -v -run TestAccess ./

# All tests
go test -v ./...

# With coverage
go test -cover ./...
```

### Unix/Linux Implementation

#### unixODBC Library Binding

```go
// File: api/api_unix.go
// Uses mksyscall_unix.pl to generate shared library bindings

package api

import "C"

/*
#cgo LDFLAGS: -lodbc
#include <sql.h>
#include <sqlext.h>
*/
```

**Library Requirements:**
```bash
# Debian/Ubuntu
sudo apt-get install libodbc1 unixodbc unixodbc-dev

# RedHat/CentOS
sudo yum install unixODBC unixODBC-devel

# macOS
brew install unixodbc
```

**Configuration Files:**
```
/etc/odbc.ini              # System-wide DSN definitions
~/.odbc.ini                # User-specific DSN definitions
/etc/odbcinst.ini          # Installed drivers
~/.odbcinst.ini            # User drivers
```

**Testing on Linux:**
```bash
# Test ODBC installation first
isql -v DSN_NAME username password

# Then run tests
go test -v -run TestMysql ./

# All tests
go test -v ./...
```

### macOS Implementation

```bash
# Install unixODBC
brew install unixodbc

# Configure DSN
nano ~/.odbc.ini

# Test connection
isql -v DSN_NAME username password

# Build and test
go build ./...
go test -v ./...
```

---

## Connection String Format

### ODBC Connection String Syntax

**Format:**
```
keyword1=value1;keyword2=value2;...
```

### Common Keywords

| Keyword | Example | Purpose |
|---------|---------|---------|
| `DSN` | `DSN=MyDatabase` | Use pre-configured DSN |
| `Driver` | `Driver={ODBC Driver 17 for SQL Server}` | Specify driver |
| `Server` | `Server=localhost` | Database server |
| `Database` | `Database=mydb` | Database name |
| `UID` | `UID=sa` | Username |
| `PWD` | `PWD=password` | Password |
| `Port` | `Port=3306` | Port number |

### Examples by Database

#### SQL Server with DSN
```
DSN=MyServer
```

#### SQL Server without DSN
```
Driver={ODBC Driver 17 for SQL Server};Server=localhost;Database=master;UID=sa;PWD=SA@12345
```

#### MySQL
```
Driver={MySQL ODBC 8.0 ANSI Driver};Server=localhost;Port=3306;Database=mydb;UID=root;PWD=password
```

#### PostgreSQL
```
Driver={PostgreSQL ANSI};Server=localhost;Port=5432;Database=mydb;UID=postgres;PWD=password
```

#### MS Access
```
Driver={Microsoft Access Driver (*.mdb, *.accdb)};DBQ=C:\data\database.accdb
```

#### Oracle
```
Driver={Oracle in OraClient19c};Server=ORCL;UID=scott;PWD=tiger
```

---

## Advanced Usage

### Context Cancellation

```go
// Timeout on query
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

rows, err := db.QueryContext(ctx, "SELECT * FROM slow_table")
if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        log.Println("Query timeout")
    }
}
```

### Named Parameters

```go
// Some ODBC drivers support named parameters
db.ExecContext(ctx,
    "UPDATE users SET name = :name WHERE id = :id",
    sql.Named("id", 42),
    sql.Named("name", "John"),
)
```

### Multiple Result Sets

```go
// Some databases (SQL Server, MySQL) support multiple result sets
rows, _ := db.Query("SELECT * FROM table1; SELECT * FROM table2")

// Process first result set
for rows.Next() {
    // Process row
}

// Get second result set
if rows.NextResultSet() {
    for rows.Next() {
        // Process row from second result set
    }
}
```

### Explicit Connection Management

```go
// Get explicit connection from pool
conn, _ := db.Conn(context.Background())
defer conn.Close()

// Ensure connection stays alive
err := conn.PingContext(context.Background())

// Use for multiple operations
rows, _ := conn.QueryContext(ctx, "SELECT ...")
```

### Connection Attributes

Advanced ODBC connection attributes can sometimes be set:

```go
// Note: Not all attributes are exposed through this driver
// For advanced tuning, may need to fork and add support
// Examples:
// - SQL_ATTR_CONNECTION_TIMEOUT
// - SQL_ATTR_LOGIN_TIMEOUT
// - SQL_ATTR_QUERY_TIMEOUT
// - SQL_ATTR_TXN_ISOLATION
```

---

## Internal Development

### Regenerating ODBC API Bindings

When modifying `api/api.go`:

```bash
cd api/
go generate

# This runs:
# Windows: go run $GOROOT/src/syscall/mksyscall_windows.go -output zapi_windows.go api.go
# Unix: sh -c "./mksyscall_unix.pl api.go | gofmt > zapi_unix.go"
```

**Never edit:**
- `api/zapi_windows.go`
- `api/zapi_unix.go`

These are auto-generated and will be overwritten.

### Adding New ODBC API Function

1. Add function signature to `api/api.go`:
```go
//sys SQLNewFunction(handle SQLHANDLE, param SQLSMALLINT) (ret SQLRETURN) = odbc32.SQLNewFunction
```

2. Regenerate:
```bash
go generate ./api
```

3. Use in code:
```go
ret := api.SQLNewFunction(handle, param)
```

### Testing with Specific Database

```bash
# Set environment variable for database
export ODBC_TEST_DSN=MyDSN

# Run tests
go test -v -run TestMysql ./

# Check test results
# Look for skip/fail output indicating DSN configuration issues
```

### Code Organization Rules

1. **Public API** (exported functions): Main `odbc/` package
2. **ODBC bindings** (C interop): `api/` package only
3. **Implementation** (internal): Unexported functions in main package
4. **Platform code**: Separate `_windows.go` and `_unix.go` files
5. **Generated code**: `zapi_*.go` files (never edited)

### Memory Leak Prevention

Checklist for handle management:

```go
// ✅ CORRECT: Always pair allocate with release
h, _ := SQLAllocHandle(...)
defer releaseHandle(h)

// ❌ WRONG: Missing cleanup
h, _ := SQLAllocHandle(...)
// Oops, forgot to release

// ✅ CORRECT: Error path cleanup
h, _ := SQLAllocHandle(...)
if err := doSomething(); err != nil {
    defer releaseHandle(h)  // Still released on error
    return err
}
```

---

## Debugging Guide

### Enable ODBC Tracing (Windows)

1. Open ODBC Administrator
2. Tracing tab
3. Enable tracing
4. Set trace file path
5. Click "OK"
6. Run application
7. Examine trace file

### Enable unixODBC Tracing (Linux)

```bash
# Edit ~/.odbc.ini or /etc/odbc.ini
[ODBC]
Trace=Yes
TraceFile=/tmp/odbc.log
TracePid=Yes
TraceOptions=3

# Run application, check /tmp/odbc.log
```

### Common Debugging Issues

#### Issue: "DSN not found"
```go
// Solution: Verify DSN is registered
// Windows: ODBC Administrator → User/System DSN → Find DSN
// Linux: grep -n "DSN_NAME" ~/.odbc.ini

// Or use connection string instead of DSN
"Driver={...};Server=...;Database=...;UID=...;PWD=..."
```

#### Issue: "UTF-16 encoding errors"
```go
// Verify string contains valid UTF-8
if !utf8.ValidString(input) {
    return fmt.Errorf("invalid UTF-8 in string")
}

// Check database collation supports UTF-8
// SQL Server: ALTER DATABASE dbname COLLATE Latin1_General_100_CI_AS
```

#### Issue: "NULL values not handled"
```go
// Use sql.NullString, sql.NullInt64, etc.
var optional sql.NullString
rows.Scan(&optional)

if optional.Valid {
    fmt.Println(optional.String)
} else {
    fmt.Println("NULL value")
}
```

---

## Summary

This technical documentation covers:
- Complete architecture and module structure
- Full API reference for public and internal interfaces
- ODBC type system and conversion rules
- Implementation details of key features
- Error handling mechanisms
- Performance optimization techniques
- Platform-specific considerations
- Advanced usage patterns
- Internal development guidelines

For additional information, refer to:
- [OVERVIEW.md](OVERVIEW.md) - High-level project overview
- [AGENTS.md](AGENTS.md) - AI development guidelines
- [Official Wiki](https://github.com/alexbrainman/odbc/wiki)
- Microsoft ODBC Programmer's Reference
