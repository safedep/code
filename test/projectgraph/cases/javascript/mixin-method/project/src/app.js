const { Loading } = require('./mixins');

class Model {}

class App extends Loading(Model) {
  start() {
    return this.load('a: 1');
  }
}

new App().start();
