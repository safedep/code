<?php
namespace App\Util;

function notify(string $m): void
{
    unlink($m);
}
