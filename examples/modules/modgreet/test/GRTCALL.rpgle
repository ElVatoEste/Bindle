**free
// GRTCALL — end-to-end caller for the modgreet demo.
// Binds the Bindle-built service program and calls its exported procedure,
// writing the result to the *DTAARA GRTRES so a batch DSPDTAARA can read it.
//
// Build (on the host, after `bindle build`/deploy of modgreet):
//   CRTDTAARA DTAARA(<lib>/GRTRES) TYPE(*CHAR) LEN(60)
//   CRTBNDDIR BNDDIR(<lib>/GRTBD)
//   ADDBNDDIRE BNDDIR(<lib>/GRTBD) OBJ((<lib>/<srvpgm> *SRVPGM))
//   CRTBNDRPG PGM(<lib>/GRTCALL) SRCSTMF('.../GRTCALL.rpgle') DFTACTGRP(*NO)
// Run: CALL <lib>/GRTCALL ; then DSPDTAARA <lib>/GRTRES

ctl-opt dftactgrp(*no) bnddir('GRTBD');

dcl-pr bgreet varchar(60) extproc('BGREET');
  name varchar(30) const;
end-pr;

dcl-s da char(60) dtaara('GRTRES');
da = bgreet('Bindle');
out da;
return;
