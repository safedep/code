const { load } = require('js-yaml');
const { parse } = require('./parse');

['a: 1'].map(load);
Promise.resolve('b: 2').then(parse);
