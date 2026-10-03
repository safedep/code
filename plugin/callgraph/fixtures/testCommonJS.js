var Anthropic = require('@anthropic-ai/sdk');
var EventEmitter = require('events').EventEmitter;
var debug = require('debug')('app');

var client = new Anthropic();
client.messages.create({ model: 'claude' });

var emitter = new EventEmitter();
emitter.emit('ready');
debug('started');
