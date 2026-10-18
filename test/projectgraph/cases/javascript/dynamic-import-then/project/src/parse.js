const yaml = require('js-yaml');

function parse(text) {
  return yaml.load(text);
}

module.exports = { parse };
