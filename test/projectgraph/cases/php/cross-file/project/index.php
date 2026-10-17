<?php
require __DIR__ . '/src/bootstrap.php';

use Shop\Loader;

$loader = new Loader($argv[1]);
$loader->load();
