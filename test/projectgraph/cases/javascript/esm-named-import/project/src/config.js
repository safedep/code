import { readFileSync } from 'node:fs';
import yaml from 'js-yaml';

export function parseConfig(path) {
  const text = readFileSync(path, 'utf8');
  return yaml.load(text);
}
