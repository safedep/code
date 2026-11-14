const child_process = require('child_process');
const fs = require('fs');

const hooks = [child_process.exec];
const defaults = { hook: fs.rmSync };

function run(cmd) {
  hooks[0](cmd);
}

function runHook(o = defaults) {
  o.hook('/tmp/x');
}

module.exports = { run, runHook };
