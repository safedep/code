const { program } = require('commander');
const yaml = require('js-yaml');

function convert(file) {
  return yaml.load(file);
}

program.command('convert <file>').action(convert);
program.parse();
