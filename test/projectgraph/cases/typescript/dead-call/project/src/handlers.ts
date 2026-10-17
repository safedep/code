import yaml from 'js-yaml';

export function parse(text: string): unknown {
  return JSON.parse(text);
}

export function loadOld(text: string): unknown {
  return yaml.load(text);
}
