#!/usr/bin/env bash
# run-callable.sh — proof that the Bindle-built service program is callable.
#
# Prereq: `bindle build` (and/or deploy) of modgreet put the *SRVPGM in $LIB.
# Run from the repo root once the IBM i host is reachable:
#
#   PROFILE=pub400 LIB=VATODEV1 SRVPGM=GRTBNDL \
#     bash examples/modules/modgreet/test/run-callable.sh
#
# Expected final line:
#   BINDLE-RESULT: Hello, Bindle! (from Bindle)
#
# The caller (GRTOUT) prints via C printf to stdout — captured directly over SSH,
# with no data area and no job that can hang in MSGW.
set -euo pipefail

PROFILE="${PROFILE:-pub400}"
LIB="${LIB:-VATODEV1}"
SRVPGM="${SRVPGM:-GRTBNDL}"
BINDLE="go run ./cmd/bindle"
HERE="examples/modules/modgreet/test"
REMOTE="/home/VATODEV/bindle/test"
# Git-Bash on Windows rewrites leading-slash args into Windows paths; disable it
# so remote IFS paths pass through unchanged.
export MSYS_NO_PATHCONV=1
export MSYS2_ARG_CONV_EXCL='*'

echo ">> uploading caller source"
$BINDLE put "$HERE/GRTOUT.rpgle" "$REMOTE/GRTOUT.rpgle" --profile "$PROFILE"

echo ">> binding to $LIB/$SRVPGM and compiling the caller"
$BINDLE exec --profile "$PROFILE" -- "
set -e
D=$REMOTE
setccsid 819 \$D/GRTOUT.rpgle
system \"CRTBNDDIR BNDDIR($LIB/GRTBD)\" >/dev/null 2>&1 || true
system \"ADDBNDDIRE BNDDIR($LIB/GRTBD) OBJ(($LIB/$SRVPGM *SRVPGM))\" >/dev/null 2>&1 || true
system \"DLTPGM PGM($LIB/GRTOUT)\" >/dev/null 2>&1 || true
system \"CRTBNDRPG PGM($LIB/GRTOUT) SRCSTMF('\$D/GRTOUT.rpgle') DFTACTGRP(*NO)\" | grep -iE 'placed|stopped' | head -1
"

echo ">> calling the Bindle-built service program:"
$BINDLE exec --profile "$PROFILE" --cl -- "CALL PGM($LIB/GRTOUT)"
