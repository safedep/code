import { Store } from './store';
import Job from './job';

const s = new Store();
s.load('a: 1');
new Job().run();
