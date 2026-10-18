import fs from 'fs';
const plugins = require(pluginPath);
const config = require('./config');

function load(name) {
  const lazy = require('./lazy');
  const mod = require('./mods/' + name);
  return import(`./locale/${name}`);
}

function run(code, obj, key) {
  eval(code);
  new Function(code)();
  Reflect.apply(obj.f, obj, []);
  obj[key]();
  fs.readFile = function () {};
  import('./later');
}

class View {
  @observe('x')
  render() {}
}

function broken( {
