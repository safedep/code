const yaml = require('js-yaml');

module.exports = function render(text) {
  return yaml.load(text);
};
