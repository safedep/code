import { load } from 'js-yaml';

export function parse(text) {
  return load(text);
}
