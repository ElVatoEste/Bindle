**free
// HELLOMOD — implementation module for the modgreet demo.
// Public API is exported through the HELLOSRV service program.

ctl-opt nomain;

dcl-proc greet export;
  dcl-pi *n varchar(60);
    name varchar(40) const;
  end-pi;
  return 'Hello, ' + %trim(name) + '!';
end-proc;
