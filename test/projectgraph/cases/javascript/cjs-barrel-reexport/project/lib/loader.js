const yaml = require('js-yaml');

function load(text) {
  return yaml.load(text);
}

module.exports = { load };
