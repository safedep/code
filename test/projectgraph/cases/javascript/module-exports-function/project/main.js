const parse = require('./lib/parse');
const Store = require('./lib/store');

parse('a: 1');
const store = new Store();
store.save('out');
