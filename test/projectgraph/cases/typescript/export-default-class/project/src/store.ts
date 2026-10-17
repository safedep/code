import yaml from 'js-yaml';

export const Store = class {
  load(t: string) {
    return yaml.load(t);
  }
};
