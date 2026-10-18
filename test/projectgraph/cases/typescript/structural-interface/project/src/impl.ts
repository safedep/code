import yaml from 'js-yaml';

export class YamlRunner {
  run(input: string): void {
    yaml.load(input);
  }
}
