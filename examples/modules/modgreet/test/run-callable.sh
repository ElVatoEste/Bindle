#!/usr/bin/env bash
# run-callable.sh — end-to-end "is the Bindle-built service program callable?" proof.
#
# Prereq: `bindle build` (and/or deploy) of modgreet has put the *SRVPGM in $LIB.
# Run from the repo root once the IBM i host is reachable:
#
#   PROFILE=pub400 LIB=VATODEV1 SRVPGM=GRTBNDL \
#     bash examples/modules/modgreet/test/run-callable.sh
#
# Expected final line: Hello, Bindle! (from Bindle)
#
# The caller runs through a CL wrapper that sets INQMSGRPY(*DFT) so a runtime
# escape auto-replies instead of leaving the job in MSGW (which would hang SSH).
set -euo pipefail

PROFILE="${PROFILE:-pub400}"
LIB="${LIB:-VATODEV1}"
SRVPGM="${SRVPGM:-GRTBNDL}"
BINDLE="go run ./cmd/bindle"
HERE="examples/modules/modgreet/test"
REMOTE="/home/VATODEV/bindle/test"

echo ">> uploading caller source"
$BINDLE put "$HERE/GRTCALL.rpgle" "$REMOTE/GRTCALL.rpgle" --profile "$PROFILE"

echo ">> setting up + compiling on $LIB (srvpgm $SRVPGM)"
$BINDLE exec --profile "$PROFILE" -- "
set -e
D=$REMOTE
setccsid 819 \$D/GRTCALL.rpgle
# CL runner that won't hang on a runtime inquiry
cat > \$D/GRTRUN.clle <<'CL'
PGM
  CHGJOB INQMSGRPY(*DFT)
  CALL PGM($LIB/GRTCALL)
  MONMSG MSGID(CPF0000)
PGMEND: ENDPGM
CL
setccsid 819 \$D/GRTRUN.clle
system \"CRTDTAARA DTAARA($LIB/GRTRES) TYPE(*CHAR) LEN(60)\" >/dev/null 2>&1 || true
system \"CRTBNDDIR BNDDIR($LIB/GRTBD)\" >/dev/null 2>&1 || true
system \"ADDBNDDIRE BNDDIR($LIB/GRTBD) OBJ(($LIB/$SRVPGM *SRVPGM))\" >/dev/null 2>&1 || true
system \"DLTPGM PGM($LIB/GRTCALL)\" >/dev/null 2>&1 || true
system \"CRTBNDRPG PGM($LIB/GRTCALL) SRCSTMF('\$D/GRTCALL.rpgle') DFTACTGRP(*NO)\" | grep -iE 'placed|stopped' | head -1
system \"DLTPGM PGM($LIB/GRTRUN)\" >/dev/null 2>&1 || true
system \"CRTBNDCL PGM($LIB/GRTRUN) SRCSTMF('\$D/GRTRUN.clle')\" | grep -iE 'placed|stopped' | head -1
"

echo ">> calling (via INQMSGRPY(*DFT) wrapper)"
$BINDLE exec --profile "$PROFILE" --cl -- "CALL PGM($LIB/GRTRUN)" || true

echo ">> result:"
$BINDLE exec --profile "$PROFILE" --cl -- "DSPDTAARA DTAARA($LIB/GRTRES)" | sed -n '/Value/,/END/p'
