import { danger } from 'plug';

function make() {
  return {
    onClick() {
      danger();
    },
  };
}

function Button(props) {
  return <button {...props}>go</button>;
}

export function App() {
  return <Button {...make()} />;
}
