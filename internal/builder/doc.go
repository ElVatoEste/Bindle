// SPDX-License-Identifier: GPL-3.0-or-later

// Package builder compiles module sources into ILE objects in dependency order,
// generates the *BNDDIR from the manifest, computes the *SRVPGM signature, and
// packages a SAVF. It wraps Bob (ibmi-bob) where that adds value.
package builder