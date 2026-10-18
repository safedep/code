const yaml = require('js-yaml');

function readConfig(text) {
  return yaml.load(text);
}

module.exports = { readConfig };
