const { Plug } = require('plug');

function go(cfg) {
  const q = cfg.handler;
  q.run();
}

go({ handler: new Plug() });
