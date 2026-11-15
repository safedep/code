import { danger, viaprop, onpress, b } from 'plug';
import { Other } from './Other';

function Child() {
  danger();
  return <div />;
}

const table = { Row: () => <span>{viaprop()}</span> };

function press() {
  onpress();
}

export default function App() {
  return (
    <main>
      <Child />
      <Other></Other>
      <table.Row />
      <button onClick={press}>go</button>
      <b>bold</b>
    </main>
  );
}
