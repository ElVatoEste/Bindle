**free
// GRTOUT — end-to-end caller proving the Bindle-built service program is callable.
// Binds the built *SRVPGM, calls its exported procedure, and writes the result to
// stdout via C printf (captured directly over SSH — no data area, no job that can
// hang in MSGW).
//
// Build + run (see run-callable.sh):
//   CRTBNDDIR BNDDIR(<lib>/GRTBD)
//   ADDBNDDIRE BNDDIR(<lib>/GRTBD) OBJ((<lib>/<srvpgm> *SRVPGM))
//   CRTBNDRPG PGM(<lib>/GRTOUT) SRCSTMF('.../GRTOUT.rpgle') DFTACTGRP(*NO)
//   CALL <lib>/GRTOUT     -> prints: BINDLE-RESULT: Hello, Bindle! (from Bindle)

ctl-opt dftactgrp(*no) bnddir('GRTBD');

dcl-pr bgreet varchar(60) extproc('BGREET');
  name varchar(30) const;
end-pr;

dcl-pr printf extproc('printf');
  fmt pointer value options(*string);
end-pr;

dcl-s line varchar(90);
line = 'BINDLE-RESULT: ' + %trim(bgreet('Bindle')) + x'25' + x'00'; // x'25'=LF, x'00'=NUL
printf(%addr(line) + 2);  // +2 skips the varchar length prefix
return;
