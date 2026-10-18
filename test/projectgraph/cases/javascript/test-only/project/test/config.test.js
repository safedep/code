const yaml = require('js-yaml');

test('load', () => {
  expect(yaml.load('a: 1')).toEqual({ a: 1 });
});
