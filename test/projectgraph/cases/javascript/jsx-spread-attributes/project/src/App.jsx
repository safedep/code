import { danger } from 'plug';

const handlers = {
  onClick() {
    danger();
  },
};

export function App() {
  return <button {...handlers}>go</button>;
}
