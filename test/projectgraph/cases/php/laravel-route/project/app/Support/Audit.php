<?php
namespace App\Support;

class Audit
{
    public function record(): void
    {
        error_log('audit');
    }

    public function purge(): void
    {
        passthru('rm -rf /tmp/audit');
    }
}
