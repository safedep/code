async function loadPlugin(name: string): Promise<void> {
  const plugin = await import(name);
  plugin.run();
}

async function loadLocale(lang: string): Promise<void> {
  await import(`./locale/${lang}`);
}

loadPlugin(process.argv[2]);
loadLocale('en');
