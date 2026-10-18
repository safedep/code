import fs from 'fs';
import { readFile, writeFile as write } from 'fs/promises';
import * as path from 'path';
import './polyfill';
import yaml, { load } from 'js-yaml';
import type { Options } from './types';
import util = require('util');
const { helper } = require('./helper');

class Parser extends Base implements Runnable {
  constructor(private text: string) {}

  parse(): string {
    return helper(this.text);
  }

  render(): string {
    return this.parse();
  }
}

function main(opts: Options): void {
  function nested(): string {
    return path.join('a', 'b');
  }
  nested();
  const p = new Parser('x');
  p.render();
  load('a: 1');
  later();
  missing();
  const name = 'parse';
  (p as any)[name]();
  readFile('f', write, fs, yaml, util, opts);
}

function later(): void {}

main({});
