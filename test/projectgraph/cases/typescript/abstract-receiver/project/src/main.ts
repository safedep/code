import { Base, Strict } from './parsers';

function run(parser: Base, text: string): unknown {
  return parser.parse(text);
}

run(new Strict(), 'a: 1');
