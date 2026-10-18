const yaml = require('js-yaml');

const handlers = [yaml.load, JSON.parse];

for (const handler of handlers) {
  handler('{}');
}
