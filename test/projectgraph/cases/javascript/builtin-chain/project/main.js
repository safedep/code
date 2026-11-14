const { Plug } = require('plug');

function go() {
  const p = new Plug();
  Object.assign(p, { a: 1 }).run();
}

go();
