// Package installer performs end-to-end installs: resolve, fetch, restore
// objects (RSTLIB/RSTOBJ) or recompile, apply DB migrations in order, validate
// signatures against the lock, and wire *BNDDIR and the runtime library list.
package installer
