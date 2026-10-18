const yaml = require('js-yaml');

class YamlParser {
  parse(text) {
    return yaml.load(text);
  }
}

const json = {
  parse: function (text) {
    return text;
  },
};

const plain = {};
plain.parse = (text) => text;

function parse(text) {
  return text;
}

module.exports = { YamlParser, json, plain };
