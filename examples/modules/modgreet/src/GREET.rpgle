**free
// Demo module for Bindle: a single exported procedure.
ctl-opt nomain;

dcl-proc bgreet export;
  dcl-pi *n varchar(60);
    name varchar(30) const;
  end-pi;
  return 'Hello, ' + %trim(name) + '! (from Bindle)';
end-proc;
