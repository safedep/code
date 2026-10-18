import fs from 'fs';
import { readFile, writeFile as write } from 'fs/promises';
import * as path from 'path';
import './polyfill';
import yaml, { load } from 'js-yaml';
const util = require('util');
const { helper } = require('./helper');

class Parser extends Base {
  constructor(text) {
    super();
    this.text = text;
  }

  parse() {
    return helper(this.text);
  }

  render() {
    return this.parse();
  }
}

function main() {
  function nested() {
    return path.join('a', 'b');
  }
  nested();
  const p = new Parser('x');
  p.render();
  load('a: 1');
  later();
  missing();
  const name = 'parse';
  p[name]();
  readFile('f', write, fs, yaml, util);
}

function later() {}

main();
