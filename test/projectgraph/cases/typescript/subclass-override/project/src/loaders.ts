import yaml from 'js-yaml';

export class Loader {
  parse(text: string): unknown {
    return text;
  }
}

export class YamlLoader extends Loader {
  parse(text: string): unknown {
    return yaml.load(text);
  }
}
