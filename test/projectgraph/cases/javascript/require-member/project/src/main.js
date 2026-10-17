const text = require('fs').readFileSync('config.yaml', 'utf8');

require('js-yaml').load(text);
