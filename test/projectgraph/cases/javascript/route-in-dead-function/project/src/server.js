const express = require('express');
const toml = require('toml');
const yaml = require('js-yaml');

const app = express();

function legacyRoutes() {
  toml.parse('dead');
  app.get('/old', (req, res) => res.json(yaml.load('a')));
}

app.listen(3000);
