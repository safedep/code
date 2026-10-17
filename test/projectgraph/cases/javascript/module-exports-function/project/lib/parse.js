const yaml = require('js-yaml');

module.exports = function (text) {
  return yaml.load(text);
};
