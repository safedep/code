import { readFileSync } from 'node:fs';
import yaml from 'js-yaml';
import type { Config } from './types';

export function parseConfig(path: string): Config {
  const text = readFileSync(path, 'utf8');
  return yaml.load(text) as Config;
}
