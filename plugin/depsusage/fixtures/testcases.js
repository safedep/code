// Importing entire Modules

// Default import
import express from 'express';
import DotEnv from 'dotenv';
const buffer = require('buffer');
let Cluster = require('cluster');
const EslintConfig = require('@gilbarbara/eslint-config');

const app = express();
if (Cluster.isMaster) {
    console.log('Master process running');
}
EslintConfig.rules.noUnusedImports = true;

// From file
import config from './config.js';
const utils = require('./utils.js');

console.log(config.serverPort);
utils.logMessage('Logging from utils.js');

// Relative import
import helper from '../utils/helper.js';
const sideeffects = require('../utils/sideeffects.js');

helper.doSomething();
sideeffects.performSideEffect();

// Import a JSON file
import jsonData from './data1.json';
const jsonData2 = require('./data2.json');

console.log(jsonData.name);
console.log(jsonData2.version);

// Wildcard import (namespace import with an alias)
import * as lodash from 'lodash';
import * as mathUtils from './math-utils';

const numbers = [1, 2, 3, 4];
const sum = lodash.sum(numbers);
console.log(mathUtils.add(10, 20));

// Using import function
const dynamicModule = await import('./dynamic-module.js');

dynamicModule.default();

// Mixed default and specified imports
import ReactDOM, { render, flushSync as flushIt } from 'react-dom';

flushIt(() => {
    render(<App />, document.getElementById('root'));
}); // Using flushSync and render from react-dom
ReactDOM.render(<App />, document.getElementById('root'));

// Importing specified items from a module

// Named import
import { EADDRINUSE, EACCES, EAGAIN } from 'constants';
import { hex } from 'chalk/ansi-styles';
const { patch } = require('virtual-dom');
const a = require("@xyz/abc"), b = require("@xyz/xyz");

console.log(`Error: ${EADDRINUSE}`);
console.log(hex.open, b.something);
patch(oldTree, newTree);

// Aliased named import
import { useEffect, useState as useMyState } from 'react';
const { bar, foo: fooAlias } = require('@xyz/pqr');
let { baz, baz2: baz2Alias } = require('@xyz/mno');

const [count, setCount] = useMyState(0);
useEffect(() => {}, []);

console.log(fooAlias(), bar());


// Commonly found in bundled js code
// imports mixed with other stuff including usage
let
    c = require("polyfill"),
    {d, baz: bazAlias} = require("vue"),
    xyz=DotEnv.config();
