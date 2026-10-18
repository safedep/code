const yaml = require('js-yaml');

const Loading = (Base) => class extends Base {
  load(text) {
    return yaml.load(text);
  }
};

module.exports = { Loading };
