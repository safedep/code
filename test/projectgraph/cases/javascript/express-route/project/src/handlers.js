const yaml = require('js-yaml');

function loadConfig(req, res) {
  res.json(yaml.load(req.body));
}

module.exports = { loadConfig };
