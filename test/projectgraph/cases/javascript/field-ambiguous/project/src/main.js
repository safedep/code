const handlers = require('./handlers');

function dispatch(handler, text) {
  return handler.handle(text);
}

dispatch(handlers[0], 'a: 1');
