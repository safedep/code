import { load } from 'store-kit';

function read(source) {
  return source.get('key');
}

const box = load();
read(box);
box.close();
