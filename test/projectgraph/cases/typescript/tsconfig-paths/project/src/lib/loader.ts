import * as yaml from 'js-yaml';

export function load(text: string): unknown {
  return yaml.load(text);
}
