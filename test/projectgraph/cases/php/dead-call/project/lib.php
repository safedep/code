<?php
function print_report(string $name): void
{
    echo htmlspecialchars($name);
}

function upload_report(string $name): void
{
    exec('scp ' . $name . ' host:');
}
