const yaml = require('js-yaml');

const api = {
  run(text) {
    return yaml.load(text);
  },
};

api.run('a: 1');
