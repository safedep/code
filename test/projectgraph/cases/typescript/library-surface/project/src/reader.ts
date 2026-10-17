import yaml from 'js-yaml';

export function readConfig(text: string): unknown {
  return yaml.load(text);
}
