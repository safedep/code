const express = require('express');
const { loadConfig } = require('./handlers');

const app = express();
const router = express.Router();

router.get('/config', loadConfig);
app.use('/api', router);
app.post('/inline', (req, res) => {
  res.json({});
});
