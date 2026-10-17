const yaml = require('js-yaml');

class Loader {
  run(text) {
    return this.parse(text);
  }

  parse(text) {
    return text;
  }
}

class YamlLoader extends Loader {
  parse(text) {
    return yaml.load(text);
  }
}

module.exports = { Loader, YamlLoader };
