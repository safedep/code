import { Loader } from './loaders';

function run(loader: Loader, text: string): unknown {
  return loader.parse(text);
}

run(new Loader(), 'a: 1');
