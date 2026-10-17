function loadPlugin(name) {
  const plugin = require(name);
  return plugin.run();
}

loadPlugin(process.argv[2]);
