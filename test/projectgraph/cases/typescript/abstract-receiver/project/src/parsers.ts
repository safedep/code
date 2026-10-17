import yaml from 'js-yaml';

export abstract class Base {
  abstract parse(text: string): unknown;
}

export class Strict extends Base {
  parse(text: string): unknown {
    return yaml.load(text);
  }
}
