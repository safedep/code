const yaml = require('js-yaml');

exports.handler = async (event) => {
  return yaml.load(event.body);
};
