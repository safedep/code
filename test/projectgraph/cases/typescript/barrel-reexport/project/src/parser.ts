import { load } from 'js-yaml';

export class Parser {
  parse(text: string): unknown {
    return load(text);
  }
}
