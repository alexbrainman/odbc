# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

- **Name**: ODBC Driver for Go
- **Type**: Database Driver Library
- **Primary Language**: Go
- **Framework**: Standard Go `database/sql` Driver Interface
- **Description**: A cross-platform ODBC database driver that implements the `database/sql` driver interface. It provides seamless access to ODBC-compliant databases (SQL Server, MySQL, MS Access, FoxPro, etc.) from Go applications. The driver uses CGO to interface with the native ODBC stack on Windows and unixODBC on Unix/Linux systems.

## Architecture & Structure

This is a **CGO-based database driver** with platform-specific implementations:

```
odbc/
├── api/                          # ODBC C API bindings layer
│   ├── api.go                    # API definitions and struct types
│   ├── api_windows.go            # Windows-specific syscall setup
│   ├── api_windows_386.go        # Windows 32-bit specific constants
│   ├── api_windows_amd64.go      # Windows 64-bit specific constants
│   ├── api_unix.go               # Unix/Linux unixODBC bindings
│   ├── zapi_windows.go           # Generated Windows syscall code (DO NOT EDIT)
│   └── zapi_unix.go              # Generated Unix syscall code (DO NOT EDIT)
├── driver.go                     # Driver initialization and registration
├── conn.go                       # Connection management
├── stmt.go                       # Statement wrapper (public driver API)
├── odbcstmt.go                   # ODBC statement implementation (internal)
├── rows.go                       # Result rows handling
├── result.go                     # Execution result handling
├── tx.go                         # Transaction support
├── param.go                      # Parameter binding logic
├── column.go                     # Column metadata handling
├── error.go                      # Error parsing and formatting
├── handle.go                     # ODBC handle lifecycle management
├── utf16.go                      # UTF-16 encoding utilities
├── stats.go                      # Statistics and metrics tracking
├── access_test.go                # MS Access integration tests
├── foxpro_test.go                # FoxPro integration tests
├── mssql_test.go                 # SQL Server integration tests
├── mysql_test.go                 # MySQL integration tests
├── go.mod                        # Module definition with minimal dependencies
└── README.md                     # Project documentation
```

### Architecture Layers

1. **Public Driver Layer** (`driver.go`, `conn.go`, `stmt.go`, `rows.go`, `result.go`, `tx.go`)
   - Implements `database/sql/driver` interfaces
   - Handles connection pooling and lifecycle
   - Provides statement preparation, execution, and result handling
   - Supports transactions and context cancellation

2. **Internal ODBC Layer** (`odbcstmt.go`, `param.go`, `column.go`)
   - Raw ODBC statement operations
   - Parameter binding and result binding
   - Column metadata retrieval and type conversion
   - Database-specific handling (MS Access special cases)

3. **C API Binding Layer** (`api/api.go` and platform-specific files)
   - CGO syscall bindings to ODBC32.DLL (Windows) or libodbcinst (Unix)
   - UTF-16 encoding/decoding for Unicode support
   - ODBC type definitions and constants
   - Handle management for environments, connections, and statements

4. **Error Handling Layer** (`error.go`)
   - ODBC diagnostic record extraction
   - Error message formatting with state codes
   - Connection state detection for bad connections

## Technology Stack

### Core Dependencies
- **Go**: 1.11+ (uses standard `database/sql` driver interface)
- **github.com/go-ole/go-ole**: v1.2.5 - COM interface support (Windows-specific utilities)
- **golang.org/x/sys**: System-level calls and syscall generation

### Platform-Specific Dependencies
- **Windows**: odbc32.dll (built-in Windows ODBC system library)
- **Unix/Linux**: unixODBC library (libodbcinst, libodbc)

### Supported Databases
Via ODBC compatibility:
- Microsoft SQL Server
- MySQL
- Microsoft Access
- FoxPro
- Any ODBC-compliant database

### Development Tools
- **CGO**: C code compilation and linking
- **Go generate**: Automatic syscall code generation
  - `mksyscall_windows.go`: Generates Windows API bindings
  - `mksyscall_unix.pl`: Generates Unix ODBC bindings

## Development Guidelines

### Code Style & Conventions

1. **Naming Conventions**
   - ODBC API functions prefixed with `SQL` (e.g., `SQLAllocHandle`, `SQLExecute`)
   - ODBC types prefixed with `SQL` (e.g., `SQLHSTMT`, `SQLHDBC`, `SQLRETURN`)
   - Package `api` contains all ODBC C bindings
   - Internal types (prefixed with unexported names) for implementation details

2. **Error Handling**
   - All ODBC API calls return `SQLRETURN` codes
   - Use `IsError()` helper to check for failures
   - Always call `NewError()` with the appropriate handle for detailed diagnostics
   - Connection state code "08S01" maps to `driver.ErrBadConn`

3. **Memory Management**
   - All ODBC handles must be released via `releaseHandle()`
   - UTF-16 encoding/decoding in boundary with C API
   - Unsafe pointers used only for C API interop in `api` package
   - sync.Mutex used for statement-level synchronization

4. **Platform Differences**
   - Windows uses `api_windows*.go` with direct syscall bindings
   - Unix/Linux uses `api_unix.go` with unixODBC library via CGO
   - Architecture-specific constants (32-bit vs 64-bit) in separate files
   - Test files document database-specific behavior

### Testing Strategy

The project uses integration testing with multiple databases:

```bash
# MS Access database tests (Windows only)
go test -v -run TestAccess ./

# FoxPro database tests
go test -v -run TestFoxpro ./

# SQL Server tests
go test -v -run TestMssql ./

# MySQL tests
go test -v -run TestMysql ./
```

**Testing Approach:**
- Real database connections (requires configured ODBC DSN)
- Tests verify database-specific functionality and edge cases
- No mock databases - relies on actual ODBC drivers installed
- Tests check parameter binding, data type conversion, and result handling

### Git Workflow

**Commit Message Format:**
- Use conventional commits when no Jira ticket applies
- Examples: `fix: handle NULL values in string parameters`, `docs: update connection pooling documentation`
- Reference specific ODBC issues when applicable

**Branch Protection:**
- Direct commits to main should include comprehensive testing
- New features require corresponding test cases for at least one database
- Breaking changes require careful testing across multiple database targets

### Build & Deployment

**CGO Compilation Requirements:**
```bash
# Windows (requires Visual C++ Build Tools)
go build ./...

# Unix/Linux (requires unixODBC development files)
# Install: apt-get install unixodbc-dev (Debian/Ubuntu)
#          brew install unixodbc (macOS)
go build ./...
```

**Generated Files:**
- `api/zapi_windows.go`: Auto-generated, run `go generate ./api` to regenerate
- `api/zapi_unix.go`: Auto-generated, run `go generate ./api` to regenerate
- Never edit generated files directly; modify `api/api.go` and regenerate

## AI Agent Instructions

### Project Context
- **Working Style**: Professional production-ready database driver development
- **Focus Areas**:
  - Reliability and correctness of ODBC operations
  - Cross-platform compatibility (Windows, Linux, macOS)
  - Memory safety and handle lifecycle management
  - Database compatibility and edge case handling
- **Common Tasks**:
  - Fixing connection pooling issues
  - Adding support for new ODBC data types
  - Improving error messages and diagnostics
  - Optimizing parameter binding
  - Adding database-specific workarounds

### Command Preferences

**File Operations:**
- Preserve existing code structure and patterns
- Be extremely careful with `api/` package changes - affects all platforms
- Platform-specific code in separate files (`_windows.go`, `_unix.go`)
- Never edit generated files (`zapi_*.go`)

**Testing:**
- Before modifying, check which databases are affected
- Integration tests are the primary validation method
- Test with multiple database targets when possible
- Consider Windows vs Unix implications for any API changes

**Building:**
- Requires CGO setup for the target platform
- Windows builds need Visual C++ Build Tools
- Unix builds need unixODBC development libraries
- Generated code must be refreshed when api.go changes: `go generate ./api`

### Best Practices for This Project

1. **ODBC Handle Management**
   - Always pair `SQLAllocHandle` with `releaseHandle()`
   - Check initialization errors early in driver startup
   - Use deferred releases for error paths

2. **Type Conversion**
   - Handle NULL values explicitly (SQL_NULL_DATA = -1)
   - Support wide Unicode (UTF-16) for string operations
   - Consider date/time precision differences between databases

3. **Connection Pooling**
   - Driver enables connection pooling at environment level
   - Set `SQL_ATTR_CONNECTION_POOLING` and `SQL_ATTR_CP_MATCH` appropriately
   - Users should call `SetMaxIdleConns(0)` for connection management

4. **Parameter Binding**
   - Use `SQLBindParameter` for prepared statement parameters
   - Handle different input/output types (SQL_PARAM_INPUT, etc.)
   - Consider decimal precision and numeric type conversions

5. **Error Diagnostics**
   - Always retrieve full diagnostic records (loop while SQLGetDiagRec returns data)
   - Include state codes in user-facing errors
   - Log native error codes for debugging

6. **Platform-Specific Handling**
   - MS Access driver requires special DSN detection
   - Different ODBC versions have different capabilities
   - Test changes on target platforms before committing

## Quick Reference

### Common Commands

```bash
# Build (requires CGO and platform dependencies)
go build ./...

# Run all tests (requires ODBC DSN configuration)
go test -v ./...

# Generate ODBC API bindings after modifying api.go
go generate ./api

# Run specific database tests
go test -v -run TestMssql ./

# Check code
go vet ./...
go fmt ./...

# Get test coverage
go test -cover ./...
```

### Wibey Commands Most Useful Here

- `/status` - Check Wibey setup and current model
- `/model` - Switch to model appropriate for CGO/C interop issues
- `!go build ./...` - Build the driver
- `!go test -v -run Test<Database> ./` - Run database-specific tests
- `!go generate ./api` - Regenerate ODBC API bindings
- `/tools` - View available development tools

### Key File Locations

- **Driver Registration**: `driver.go` (lines 72-78)
- **ODBC API Definitions**: `api/api.go` (type definitions and `//sys` comments)
- **Connection Management**: `conn.go` (connection lifecycle)
- **Statement Execution**: `stmt.go` (public interface) and `odbcstmt.go` (implementation)
- **Error Handling**: `error.go` (diagnostic record extraction)
- **Parameter Binding**: `param.go` (type conversion and binding)

## Notes & Context

### Architectural Decisions

1. **CGO Boundary at api/ Package**
   - Keeps unsafe code isolated and well-defined
   - Easy to audit for memory safety issues
   - Clear separation between Go and C concerns

2. **Connection Pooling at Environment Level**
   - ODBC 3.0 feature automatically enabled
   - Relaxed matching for maximum compatibility
   - Users control idle connection limits via `SetMaxIdleConns()`

3. **UTF-16 Everywhere for C API**
   - ODBC on Windows is Unicode-native (UTF-16)
   - Unix ODBC with UTF-16 for consistency
   - Conversion happens at package boundaries

4. **Platform-Specific Architecture Constants**
   - Separate files for 32-bit and 64-bit Windows
   - Ensures correct pointer sizes in ODBC structs
   - Unix/Linux unified (64-bit typically)

### Integration Points

- **Calls Into**: Windows ODBC32.DLL, unixODBC library (libodbc)
- **Called By**: Any Go application using `database/sql` with `odbc` driver registration
- **Data Formats**: UTF-16 for C API, UTF-8 for Go API
- **Handle Types**: Environment (ENV), Connection (DBC), Statement (STMT)

### Performance Considerations

1. **Connection Pooling**
   - Enabled at driver initialization
   - Reduces overhead of repeated connect/disconnect
   - Users should tune `SetMaxIdleConns()` and `SetMaxOpenConns()`

2. **Type Conversion**
   - Column binding minimizes data copies
   - Large text data retrieved with SQLGetData (chunking)
   - Numeric and date conversions done at binding time

3. **Statement Preparation**
   - Prepared statements reduce database server load
   - Parameter binding with direct data types when possible
   - Cursor management for result set iteration

### Common Debugging Areas

1. **ODBC DSN Not Found**: Verify DSN is configured in system ODBC settings
2. **UTF-16 Encoding Issues**: Check string conversion at C API boundary
3. **NULL Handling**: SQL_NULL_DATA (-1) must be checked explicitly
4. **Connection State**: "08S01" diagnostic indicates connection lost
5. **Parameter Binding**: Ensure parameter count matches prepared statement
6. **Database-Specific Quirks**: Check test files for known workarounds (e.g., Access driver detection)

### Security Considerations

- **Input Validation**: Use parameterized queries (prepared statements) for all user input
- **Connection Strings**: DSN-based connection preferred over embedded credentials
- **Diagnostic Messages**: Error messages don't expose passwords or sensitive data
- **ODBC Driver Selection**: Only trusted ODBC drivers should be installed system-wide

### Future Enhancement Areas

- Thread-safe statement pool for high-concurrency scenarios
- Async query execution support
- Built-in connection string encryption
- Enhanced type mapping for JSON/XML columns
- Performance profiling integration
