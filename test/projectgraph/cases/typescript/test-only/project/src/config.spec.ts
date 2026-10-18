import yaml from 'js-yaml';

describe('config', () => {
  it('loads', () => {
    expect(yaml.load('a: 1')).toEqual({ a: 1 });
  });
});
