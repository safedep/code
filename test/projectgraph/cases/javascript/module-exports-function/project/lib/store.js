const fs = require('fs');

module.exports = class {
  save(path) {
    fs.writeFileSync(path, 'x');
  }
};
