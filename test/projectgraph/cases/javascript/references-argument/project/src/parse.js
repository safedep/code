const yaml = require('js-yaml');

function parse(text) {
  return yaml.dump(yaml.loadAll(text));
}

module.exports = { parse };
