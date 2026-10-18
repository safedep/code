const yaml = require('js-yaml');

class Handler {
  handle(text) {
    return yaml.load(text);
  }

  run(text) {
    const self = this;
    return self.handle(text);
  }
}

new Handler().run('a: 1');
