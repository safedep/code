// CommonJS forms that older Node code and bundles use

var Router = require('router');
var { json, urlencoded: urlEncoded } = require('body-parser');
var EventEmitter = require('events').EventEmitter;
var debug = require('debug')('express:application');
const { default: JSZip, generate } = await import('jszip');
mixin = require('merge-descriptors');
Stream = require('stream').Stream;
log = require('pino')({ level: 'info' });
const env = require('dotenv-flow').config();
const require = load('settings').value;

// Loaded for side effects, with no binding
import 'reflect-metadata';
require('dotenv').config();
require('./register-hooks');
