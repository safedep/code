const yaml = require('js-yaml');

function parse(text) {
  return JSON.parse(text);
}

function loadOld(text) {
  return yaml.load(text);
}

module.exports = { parse, loadOld };
