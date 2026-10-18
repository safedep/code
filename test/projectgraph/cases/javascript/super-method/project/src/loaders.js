const yaml = require('js-yaml');

class Base {
  load(text) {
    return yaml.load(text);
  }
}

class Strict extends Base {
  load(text) {
    return super.load(text);
  }
}

module.exports = { Base, Strict };
