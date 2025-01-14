// Importing entire Modules

// Default import
import express from 'express';
import DotEnv from 'dotenv'
const buffer = require('buffer');
const Cluster = require('cluster');
const EslintConfig = require('@gilbarbara/eslint-config');

// From file
import config from './config.js';
const utils = require('./utils.js');

// Relative import
import helper from '../utils/helper.js';
const sideffects = require('../utils/sideeffects.js');

// Import a JSON file
import jsonData from './data1.json';
const jsonData2 = require('./data2.json');

// Wildcard import (namespace import with an alias)
import * as lodash from 'lodash';
import * as mathUtils from './math-utils'; // remaining



// using import function
const dynamicModule = await import('./dynamic-module.js');

// Mixed default and specified imports
import ReactDOM, { render, flushSync as flushIt } from 'react-dom';



// Importing specified items from a module

// Named import
import { EADDRINUSE, EACCES, EAGAIN } from 'constants';
import { hex } from 'chalk/ansi-styles';
const { patch } = require('virtual-dom');

// Aliased named import
import { useEffect, useState as useMyState } from 'react';
const { bar, foo: fooAlias } = require('@xyz/pqr');
