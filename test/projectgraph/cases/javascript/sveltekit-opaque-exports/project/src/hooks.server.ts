import { sequence } from '@sveltejs/kit/hooks';
import { h1, h2 } from 'plug';

const first = async ({ event, resolve }) => {
  h1();
  return resolve(event);
};

async function second({ event, resolve }) {
  h2();
  return resolve(event);
}

export const handle = sequence(first, second);
