import fs from 'fs';
const plugins = require(pluginPath);

function load(name: string) {
  const lazy = require('./lazy');
  return import(`./locale/${name}`);
}

function run(code: string, obj: any, key: string) {
  eval(code);
  Function(code)();
  Reflect.get(obj, key);
  obj[key]();
  (fs as any).readFile = () => {};
  fs.writeFile = () => {};
}

class View {
  @Input() name: string = '';

  @observe('x')
  render(): void {}
}

function broken( {
