const { YamlParser } = require('./parsers');

function run(parser, text) {
  return parser.parse(text);
}

run(new YamlParser(), 'a: 1');
