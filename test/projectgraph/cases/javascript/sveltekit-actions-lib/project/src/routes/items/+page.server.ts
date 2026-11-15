import type { Actions } from './$types';
import { a6 } from 'plug';

export const actions = {
  default: async () => {
    a6();
  },
} satisfies Actions;
